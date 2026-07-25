package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/ipfs/go-cid"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/protocol"
)

const (
	protocolID     = protocol.ID("/secnet/chat/1.0.0")
	fileProtocolID = protocol.ID("/secnet/file/1.0.0")
	chunkSize      = 8192
)

type ChatProtocol struct {
	host      host.Host
	dht       *dht.IpfsDHT
	app       *App
	streams   map[string]network.Stream
	streamsMu sync.Mutex
	writeMu   sync.Mutex
}

func NewChatProtocol(h host.Host, d *dht.IpfsDHT, a *App) *ChatProtocol {
	cp := &ChatProtocol{
		host:    h,
		dht:     d,
		app:     a,
		streams: make(map[string]network.Stream),
	}
	h.SetStreamHandler(protocolID, cp.HandleStream)
	h.SetStreamHandler(fileProtocolID, cp.HandleFileStream)
	return cp
}

func (cp *ChatProtocol) ProtocolID() protocol.ID {
	return protocolID
}

func (cp *ChatProtocol) getStream(pid peer.ID) (network.Stream, error) {
	cp.streamsMu.Lock()
	pidStr := pid.String()
	if s, ok := cp.streams[pidStr]; ok {
		cp.streamsMu.Unlock()
		return s, nil
	}
	cp.streamsMu.Unlock()

	s, err := cp.host.NewStream(context.Background(), pid, protocolID)
	if err != nil {
		return nil, err
	}

	cp.streamsMu.Lock()
	cp.streams[pidStr] = s
	cp.streamsMu.Unlock()

	go cp.readLoop(s, pidStr)

	return s, nil
}

func (cp *ChatProtocol) closeStream(pid peer.ID) {
	cp.streamsMu.Lock()
	defer cp.streamsMu.Unlock()

	pidStr := pid.String()
	if s, ok := cp.streams[pidStr]; ok {
		s.Close()
		delete(cp.streams, pidStr)
	}
}

func (cp *ChatProtocol) HandleStream(s network.Stream) {
	pid := s.Conn().RemotePeer()
	pidStr := pid.String()

	cp.streamsMu.Lock()
	cp.streams[pidStr] = s
	cp.streamsMu.Unlock()

	cp.readLoop(s, pidStr)
}

func (cp *ChatProtocol) HandleFileStream(s network.Stream) {
	pid := s.Conn().RemotePeer()
	pidStr := pid.String()

	reader := bufio.NewReader(s)

	fileName, err := reader.ReadString('\n')
	if err != nil {
		return
	}
	fileName = strings.TrimSuffix(fileName, "\n")

	sizeStr, err := reader.ReadString('\n')
	if err != nil {
		return
	}
	var fileSize int64
	fmt.Sscanf(strings.TrimSuffix(sizeStr, "\n"), "%d", &fileSize)

	app := cp.app

	var data []byte
	buf := make([]byte, chunkSize)
	for int64(len(data)) < fileSize {
		n, err := reader.Read(buf)
		if err != nil {
			break
		}
		if n == 0 {
			break
		}
		data = append(data, buf[:n]...)
	}

	app.addFileMessage(pidStr, app.host.ID().String(), fileName, fileSize, data)
	app.markDelivered(pidStr, app.host.ID().String())
	fmt.Printf("Received file: %s from %s (click to download)\n", fileName, pidStr)
	s.Close()
}

func (cp *ChatProtocol) readLoop(s network.Stream, pidStr string) {
	pid, _ := peer.Decode(pidStr)
	defer func() {
		cp.closeStream(pid)
		cp.app.removePeer(pidStr)
	}()

	cp.app.addPeer(pidStr)
	reader := bufio.NewReader(s)

	for {
		msg, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				fmt.Printf("Read error: %v\n", err)
			}
			return
		}

		msg = strings.TrimSuffix(msg, "\n")
		from := pidStr
		to := cp.host.ID().String()

		cp.app.addMessage(from, to, msg)
		cp.app.markDelivered(from, to)
	}
}

func (cp *ChatProtocol) SendMessage(ctx context.Context, targetPeer peer.ID, msg string) {
	targetStr := targetPeer.String()

	if cp.host.Network().Connectedness(targetPeer) != network.Connected {
		peerInfo, err := cp.dht.FindPeer(ctx, targetPeer)
		if err != nil {
			fmt.Printf("DHT error: %v\n", err)
			return
		}

		cp.host.Peerstore().AddAddrs(targetPeer, peerInfo.Addrs, peerstore.PermanentAddrTTL)

		connectCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()

		err = cp.host.Connect(connectCtx, peerInfo)
		if err != nil {
			fmt.Printf("Connect error: %v\n", err)
			return
		}

		cp.app.addPeer(targetStr)
	}

	s, err := cp.getStream(targetPeer)
	if err != nil {
		fmt.Printf("Stream error: %v\n", err)
		return
	}

	cp.writeMu.Lock()
	_, err = s.Write([]byte(msg + "\n"))
	cp.writeMu.Unlock()

	if err != nil {
		fmt.Printf("Write error: %v\n", err)
		cp.closeStream(targetPeer)
		return
	}

	cp.app.markDelivered(cp.host.ID().String(), targetStr)
}

func (cp *ChatProtocol) SendFile(ctx context.Context, targetPeer peer.ID, fileName string, data []byte) {
	targetStr := targetPeer.String()

	if cp.host.Network().Connectedness(targetPeer) != network.Connected {
		peerInfo, err := cp.dht.FindPeer(ctx, targetPeer)
		if err != nil {
			fmt.Printf("DHT error: %v\n", err)
			return
		}

		cp.host.Peerstore().AddAddrs(targetPeer, peerInfo.Addrs, peerstore.PermanentAddrTTL)

		connectCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()

		err = cp.host.Connect(connectCtx, peerInfo)
		if err != nil {
			fmt.Printf("Connect error: %v\n", err)
			return
		}

		cp.app.addPeer(targetStr)
	}

	s, err := cp.host.NewStream(ctx, targetPeer, fileProtocolID)
	if err != nil {
		fmt.Printf("File stream error: %v\n", err)
		return
	}
	defer s.Close()

	cp.writeMu.Lock()
	_, err = s.Write([]byte(fileName + "\n"))
	if err != nil {
		cp.writeMu.Unlock()
		return
	}
	_, err = s.Write([]byte(fmt.Sprintf("%d\n", len(data))))
	if err != nil {
		cp.writeMu.Unlock()
		return
	}
	cp.writeMu.Unlock()

	var sent int64
	for sent < int64(len(data)) {
		end := sent + chunkSize
		if end > int64(len(data)) {
			end = int64(len(data))
		}
		chunk := data[sent:end]

		cp.writeMu.Lock()
		_, err = s.Write(chunk)
		cp.writeMu.Unlock()

		if err != nil {
			fmt.Printf("File write error: %v\n", err)
			return
		}
		sent = end
	}

	cp.app.markDelivered(cp.host.ID().String(), targetStr)
	fmt.Printf("Sent file: %s to %s\n", fileName, targetStr)
}

func (cp *ChatProtocol) DiscoverPeers(ctx context.Context) {
	c, err := cid.Decode("bafkqaaa")
	if err != nil {
		return
	}

	go func() {
		for {
			_ = cp.dht.Provide(ctx, c, true)
			select {
			case <-ctx.Done():
				return
			case <-time.After(30 * time.Minute):
			}
		}
	}()

	providers, err := cp.dht.FindProviders(ctx, c)
	if err != nil {
		return
	}

	for _, peerInfo := range providers {
		if peerInfo.ID == cp.host.ID() {
			continue
		}

		cp.host.Peerstore().AddAddrs(peerInfo.ID, peerInfo.Addrs, peerstore.PermanentAddrTTL)

		err := cp.host.Connect(ctx, peerInfo)
		if err == nil {
			cp.app.addPeer(peerInfo.ID.String())
		}
	}
}