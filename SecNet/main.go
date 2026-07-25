package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

var (
	keyFile   = flag.String("key", "key.bin", "file to store private key")
	port      = flag.Int("port", 9000, "port to listen on")
	webPort   = flag.Int("web", 8080, "port for web interface")
	relayMode = flag.Bool("relay", false, "run as relay server")
)

type Message struct {
	From       string `json:"from"`
	To         string `json:"to"`
	Content    string `json:"content"`
	Time       string `json:"time"`
	Delivered  bool   `json:"delivered"`
	IsFile     bool   `json:"is_file"`
	FileName   string `json:"file_name"`
	FileSize   int64  `json:"file_size"`
	FileData   []byte `json:"-"`
	FileID     string `json:"file_id"`
	Downloaded bool   `json:"downloaded"`
}

type FileStore struct {
	mu    sync.Mutex
	files map[string][]byte
	ids   map[string]string
}

type App struct {
	host       host.Host
	dht        *dht.IpfsDHT
	protocol   *ChatProtocol
	ctx        context.Context
	cancel     context.CancelFunc
	messages   []Message
	mu         sync.Mutex
	peers      map[string]bool
	peerMu     sync.Mutex
	downloads  string
	fileStore  *FileStore
	fileMu     sync.Mutex
}

func NewFileStore() *FileStore {
	return &FileStore{
		files: make(map[string][]byte),
		ids:   make(map[string]string),
	}
}

func (fs *FileStore) Add(data []byte) string {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	id := fmt.Sprintf("%d", time.Now().UnixNano())
	fs.files[id] = data
	return id
}

func (fs *FileStore) Get(id string) ([]byte, bool) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	data, ok := fs.files[id]
	return data, ok
}

func (fs *FileStore) Remove(id string) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	delete(fs.files, id)
}

func NewApp() *App {
	ctx, cancel := context.WithCancel(context.Background())
	os.MkdirAll("downloads", 0755)
	return &App{
		ctx:       ctx,
		cancel:    cancel,
		peers:     make(map[string]bool),
		downloads: "downloads",
		fileStore: NewFileStore(),
	}
}

func (app *App) addMessage(from, to, content string) {
	app.mu.Lock()
	defer app.mu.Unlock()
	app.messages = append(app.messages, Message{
		From:      from,
		To:        to,
		Content:   content,
		Time:      time.Now().Format("15:04:05"),
		Delivered: false,
		IsFile:    false,
	})
	if len(app.messages) > 500 {
		app.messages = app.messages[1:]
	}
}

func (app *App) addFileMessage(from, to, fileName string, fileSize int64, fileData []byte) string {
	app.mu.Lock()
	defer app.mu.Unlock()
	fileID := app.fileStore.Add(fileData)
	msg := Message{
		From:       from,
		To:         to,
		Content:    "[File] " + fileName,
		Time:       time.Now().Format("15:04:05"),
		Delivered:  false,
		IsFile:     true,
		FileName:   fileName,
		FileSize:   fileSize,
		FileID:     fileID,
		Downloaded: false,
	}
	app.messages = append(app.messages, msg)
	if len(app.messages) > 500 {
		app.messages = app.messages[1:]
	}
	return fileID
}

func (app *App) markDelivered(from, to string) {
	app.mu.Lock()
	defer app.mu.Unlock()
	for i := len(app.messages) - 1; i >= 0; i-- {
		if app.messages[i].From == from && app.messages[i].To == to && !app.messages[i].Delivered {
			app.messages[i].Delivered = true
			break
		}
	}
}

func (app *App) markFileDownloaded(fileID string) {
	app.mu.Lock()
	defer app.mu.Unlock()
	for i := len(app.messages) - 1; i >= 0; i-- {
		if app.messages[i].FileID == fileID && !app.messages[i].Downloaded {
			app.messages[i].Downloaded = true
			break
		}
	}
}

func (app *App) getMessages() []Message {
	app.mu.Lock()
	defer app.mu.Unlock()
	return app.messages
}

func (app *App) addPeer(pid string) {
	app.peerMu.Lock()
	defer app.peerMu.Unlock()
	app.peers[pid] = true
}

func (app *App) removePeer(pid string) {
	app.peerMu.Lock()
	defer app.peerMu.Unlock()
	delete(app.peers, pid)
}

func (app *App) getPeers() []string {
	app.peerMu.Lock()
	defer app.peerMu.Unlock()
	result := make([]string, 0, len(app.peers))
	for pid := range app.peers {
		result = append(result, pid)
	}
	return result
}

func loadOrCreateKey(path string) (crypto.PrivKey, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		return crypto.UnmarshalPrivateKey(data)
	}
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
	if err != nil {
		return nil, err
	}
	data, err = crypto.MarshalPrivateKey(priv)
	if err != nil {
		return nil, err
	}
	err = os.WriteFile(path, data, 0600)
	if err != nil {
		return nil, err
	}
	return priv, nil
}

func getBootstrapPeers() []peer.AddrInfo {
	bootstrapPeers := []string{
		"/ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",
		"/ip4/138.197.158.185/tcp/4001/p2p/QmSoLPppuBtQSGwKDwe2x2cdbzt6nxQ7xMrQHvBWcihMQC",
	}
	var peers []peer.AddrInfo
	for _, addrStr := range bootstrapPeers {
		ma, err := multiaddr.NewMultiaddr(addrStr)
		if err != nil {
			continue
		}
		pi, err := peer.AddrInfoFromP2pAddr(ma)
		if err != nil {
			continue
		}
		peers = append(peers, *pi)
	}
	return peers
}

func getPublicRelays() []peer.AddrInfo {
	relayMultiaddrs := []string{
		"/ip4/147.75.80.54/tcp/4001/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
		"/ip4/147.75.80.110/tcp/4001/p2p/QmbFgm5zan8P6eWWmeyfncR5feYEMPbht5b1FW1C37aQ7y",
		"/ip4/147.75.195.153/tcp/4001/p2p/QmW9m57aiBDHAkKj9nmFSEn7ZqrcF1fZS4bipsTCHburei",
		"/ip4/147.75.70.221/tcp/4001/p2p/Qme8g49gm3q4Acp7xWBKg3nAa9fxZ1YmyDJdyGgoG6LsXh",
	}
	var relays []peer.AddrInfo
	for _, addrStr := range relayMultiaddrs {
		ma, err := multiaddr.NewMultiaddr(addrStr)
		if err != nil {
			continue
		}
		pi, err := peer.AddrInfoFromP2pAddr(ma)
		if err != nil {
			continue
		}
		relays = append(relays, *pi)
	}
	return relays
}

func (app *App) handleSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		PeerID  string `json:"peer_id"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	targetID, err := peer.Decode(req.PeerID)
	if err != nil {
		http.Error(w, "Invalid peer ID", http.StatusBadRequest)
		return
	}
	app.addMessage(app.host.ID().String(), targetID.String(), req.Message)
	go app.protocol.SendMessage(app.ctx, targetID, req.Message)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "sent"})
}

func (app *App) handleSendFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	peerID := r.FormValue("peer_id")
	if peerID == "" {
		http.Error(w, "peer_id required", http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	targetID, err := peer.Decode(peerID)
	if err != nil {
		http.Error(w, "Invalid peer ID", http.StatusBadRequest)
		return
	}

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	app.addFileMessage(app.host.ID().String(), targetID.String(), header.Filename, header.Size, data)
	go app.protocol.SendFile(app.ctx, targetID, header.Filename, data)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "file_sent"})
}

func (app *App) handleDownloadFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	fileID := r.URL.Query().Get("id")
	if fileID == "" {
		http.Error(w, "file id required", http.StatusBadRequest)
		return
	}

	data, ok := app.fileStore.Get(fileID)
	if !ok {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	var fileName string
	app.mu.Lock()
	for _, msg := range app.messages {
		if msg.FileID == fileID {
			fileName = msg.FileName
			break
		}
	}
	app.mu.Unlock()

	if fileName == "" {
		fileName = "file"
	}

	w.Header().Set("Content-Disposition", "attachment; filename=\""+fileName+"\"")
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	w.Write(data)

	app.markFileDownloaded(fileID)

	savePath := app.downloads + "/" + fileName
	os.WriteFile(savePath, data, 0644)
	fmt.Printf("File downloaded: %s\n", fileName)
}

func (app *App) handleList(w http.ResponseWriter, r *http.Request) {
	peers := app.getPeers()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"peers":       peers,
		"peer_id":     app.host.ID().String(),
		"connections": len(peers),
	})
}

func (app *App) handleMessages(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(app.getMessages())
}

func (app *App) handleStatus(w http.ResponseWriter, r *http.Request) {
	addrs := []string{}
	for _, addr := range app.host.Addrs() {
		addrs = append(addrs, addr.String())
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"peer_id":   app.host.ID().String(),
		"addresses": addrs,
		"connected": len(app.getPeers()),
	})
}

func (app *App) serveGUI() {
	http.HandleFunc("/api/send", app.handleSend)
	http.HandleFunc("/api/sendfile", app.handleSendFile)
	http.HandleFunc("/api/download", app.handleDownloadFile)
	http.HandleFunc("/api/list", app.handleList)
	http.HandleFunc("/api/messages", app.handleMessages)
	http.HandleFunc("/api/status", app.handleStatus)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		io.WriteString(w, htmlTemplate)
	})

	fmt.Printf("Web interface available at http://localhost:%d\n", *webPort)
	go http.ListenAndServe(fmt.Sprintf(":%d", *webPort), nil)
}

func (app *App) connectionManager() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-app.ctx.Done():
			return
		case <-ticker.C:
			connected := app.host.Network().Peers()
			currentPeers := make(map[string]bool)

			for _, p := range connected {
				if app.host.Network().Connectedness(p) == network.Connected {
					pid := p.String()
					currentPeers[pid] = true
					app.addPeer(pid)
				}
			}

			app.peerMu.Lock()
			for pid := range app.peers {
				if !currentPeers[pid] {
					delete(app.peers, pid)
				}
			}
			app.peerMu.Unlock()
		}
	}
}

func (app *App) registerInDHT(ctx context.Context, kad *dht.IpfsDHT) {
	nsCid, err := cid.Decode("bafkqaaa")
	if err != nil {
		fmt.Printf("CID decode error: %v\n", err)
		return
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-app.ctx.Done():
			return
		case <-ticker.C:
			err := kad.Provide(ctx, nsCid, true)
			if err != nil {
				fmt.Printf("DHT provide error: %v\n", err)
			}
		}
	}
}

func (app *App) discoverPeers() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-app.ctx.Done():
			return
		case <-ticker.C:
			app.protocol.DiscoverPeers(app.ctx)
		}
	}
}

func main() {
	flag.Parse()

	priv, err := loadOrCreateKey(*keyFile)
	if err != nil {
		fmt.Printf("Key error: %v\n", err)
		os.Exit(1)
	}

	tcpAddr := fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", *port)
	quicAddr := fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic-v1", *port)

	opts := []libp2p.Option{
		libp2p.Identity(priv),
		libp2p.ListenAddrStrings(tcpAddr, quicAddr),
		libp2p.DefaultTransports,
		libp2p.DefaultMuxers,
		libp2p.DefaultSecurity,
		libp2p.EnableNATService(),
		libp2p.EnableRelay(),
		libp2p.EnableHolePunching(),
	}

	if *relayMode {
		fmt.Println("RELAY SERVER MODE")
		opts = append(opts, libp2p.EnableRelayService())
	} else {
		fmt.Println("CLIENT MODE (using public relays)")
		publicRelays := getPublicRelays()
		opts = append(opts, libp2p.EnableAutoRelayWithStaticRelays(publicRelays))
	}

	host, err := libp2p.New(opts...)
	if err != nil {
		fmt.Printf("Host creation error: %v\n", err)
		os.Exit(1)
	}
	defer host.Close()

	fmt.Printf("Peer ID: %s\n", host.ID().String())
	fmt.Printf("Listening on TCP: %s | QUIC: %s\n", tcpAddr, quicAddr)

	app := NewApp()
	app.host = host

	ctx := context.Background()

	kad, err := dht.New(ctx, host, dht.BootstrapPeers(getBootstrapPeers()...))
	if err != nil {
		fmt.Printf("DHT creation error: %v\n", err)
		os.Exit(1)
	}
	app.dht = kad

	err = kad.Bootstrap(ctx)
	if err != nil {
		fmt.Printf("Bootstrap error: %v\n", err)
	}

	time.Sleep(3 * time.Second)
	fmt.Println("DHT bootstrapped and connected to global network")

	protocol := NewChatProtocol(host, kad, app)
	app.protocol = protocol

	go app.registerInDHT(ctx, kad)
	go app.discoverPeers()
	go app.connectionManager()

	if *relayMode {
		fmt.Println("RELAY SERVER IS RUNNING")
	} else {
		fmt.Println("CLIENT MODE ACTIVE")
	}

	app.serveGUI()

	fmt.Println("\nCommands:")
	fmt.Println("  send <peer_id> <message> - send a message")
	fmt.Println("  list - show connected peers")
	fmt.Println("  connect <multiaddress> - manually connect to a peer")
	fmt.Println("  status - show network status")
	fmt.Println("  exit - quit")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 3)
		switch parts[0] {
		case "exit":
			app.cancel()
			return
		case "status":
			fmt.Printf("Connected peers: %d\n", len(app.getPeers()))
			fmt.Printf("Messages in history: %d\n", len(app.getMessages()))
		case "list":
			peers := app.getPeers()
			for _, p := range peers {
				fmt.Printf("Connected: %s\n", p)
			}
			fmt.Printf("Total connected: %d\n", len(peers))
		case "connect":
			if len(parts) < 2 {
				fmt.Println("Usage: connect <multiaddress>")
				continue
			}
			pi, err := peer.AddrInfoFromString(parts[1])
			if err != nil {
				fmt.Printf("Invalid address: %v\n", err)
				continue
			}
			err = host.Connect(ctx, *pi)
			if err != nil {
				fmt.Printf("Connection error: %v\n", err)
			} else {
				fmt.Printf("Connected to %s\n", pi.ID.String())
				app.addPeer(pi.ID.String())
			}
		case "send":
			if len(parts) < 3 {
				fmt.Println("Usage: send <peer_id> <message>")
				continue
			}
			targetID, err := peer.Decode(parts[1])
			if err != nil {
				fmt.Printf("Invalid peer ID: %v\n", err)
				continue
			}
			msg := parts[2]
			app.addMessage(host.ID().String(), targetID.String(), msg)
			go protocol.SendMessage(ctx, targetID, msg)
		default:
			fmt.Println("Unknown command. Available: send, list, connect, status, exit")
		}
	}
}