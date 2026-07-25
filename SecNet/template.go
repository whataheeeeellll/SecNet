package main

const htmlTemplate = `<!DOCTYPE html>
<html>
<head>
    <title>SecNet</title>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: #1a1a1a;
            height: 100vh;
            display: flex;
            flex-direction: column;
            color: #d4d4d4;
        }
        .header {
            background: #1a1a1a;
            color: #d4d4d4;
            padding: 14px 24px;
            display: flex;
            justify-content: space-between;
            align-items: center;
            flex-shrink: 0;
            border-bottom: 1px solid #333;
        }
        .header h1 {
            font-size: 18px;
            font-weight: 500;
            color: #d4d4d4;
        }
        .header .status {
            font-size: 13px;
            color: #888;
        }
        .header .status span {
            color: #6a6;
        }
        .container {
            display: flex;
            flex: 1;
            overflow: hidden;
        }
        .sidebar {
            width: 280px;
            background: #1e1e1e;
            border-right: 1px solid #333;
            display: flex;
            flex-direction: column;
            flex-shrink: 0;
        }
        .sidebar .peer-id-box {
            padding: 14px 16px;
            background: #222;
            border-bottom: 1px solid #333;
        }
        .sidebar .peer-id-box label {
            font-size: 11px;
            color: #888;
            display: block;
            margin-bottom: 4px;
        }
        .sidebar .peer-id-box input {
            width: 100%;
            padding: 6px 10px;
            border: 1px solid #333;
            border-radius: 4px;
            font-size: 12px;
            background: #1a1a1a;
            color: #d4d4d4;
            font-family: monospace;
        }
        .sidebar .peer-list {
            flex: 1;
            overflow-y: auto;
            padding: 6px 0;
        }
        .sidebar .peer-list .title {
            padding: 8px 16px;
            font-size: 11px;
            color: #888;
            font-weight: 600;
            text-transform: uppercase;
            letter-spacing: 0.5px;
        }
        .sidebar .peer-list .peer-item {
            padding: 8px 16px;
            cursor: pointer;
            display: flex;
            align-items: center;
            gap: 10px;
            border-bottom: 1px solid #2a2a2a;
            transition: background 0.1s;
        }
        .sidebar .peer-list .peer-item:hover {
            background: #2a2a2a;
        }
        .sidebar .peer-list .peer-item .dot {
            width: 8px;
            height: 8px;
            border-radius: 50%;
            background: #6a6;
            flex-shrink: 0;
        }
        .sidebar .peer-list .peer-item .id {
            font-size: 12px;
            font-family: monospace;
            color: #d4d4d4;
            word-break: break-all;
            flex: 1;
        }
        .main {
            flex: 1;
            display: flex;
            flex-direction: column;
            background: #1a1a1a;
        }
        .messages {
            flex: 1;
            overflow-y: auto;
            padding: 16px 24px;
            background: #1a1a1a;
        }
        .messages .message {
            margin-bottom: 10px;
            padding: 10px 14px;
            border-radius: 6px;
            max-width: 70%;
            word-wrap: break-word;
            font-size: 14px;
            line-height: 1.5;
        }
        .messages .message.incoming {
            background: #2a2a2a;
            border: 1px solid #333;
            align-self: flex-start;
            margin-right: auto;
            color: #d4d4d4;
        }
        .messages .message.outgoing {
            background: #333;
            color: #d4d4d4;
            align-self: flex-end;
            margin-left: auto;
        }
        .messages .message .meta {
            font-size: 11px;
            opacity: 0.6;
            margin-top: 4px;
        }
        .messages .message.incoming .meta {
            color: #888;
        }
        .messages .message.outgoing .meta {
            color: #888;
        }
        .messages .message .delivered {
            color: #6a6;
            font-size: 11px;
            margin-left: 8px;
        }
        .messages .message .not-delivered {
            color: #aa6;
            font-size: 11px;
            margin-left: 8px;
        }
        .messages .message .file-link {
            color: #88bbff;
            cursor: pointer;
            text-decoration: underline;
        }
        .messages .message .file-link:hover {
            color: #aaccee;
        }
        .messages .message .file-downloaded {
            color: #6a6;
            font-size: 11px;
            margin-left: 8px;
        }
        .input-area {
            padding: 12px 24px;
            border-top: 1px solid #333;
            display: flex;
            gap: 10px;
            background: #1a1a1a;
            flex-shrink: 0;
            flex-wrap: wrap;
        }
        .input-area input {
            flex: 1;
            padding: 10px 14px;
            border: 1px solid #333;
            border-radius: 4px;
            font-size: 14px;
            outline: none;
            transition: border 0.2s;
            background: #222;
            color: #d4d4d4;
            min-width: 120px;
        }
        .input-area input:focus {
            border-color: #555;
        }
        .input-area input::placeholder {
            color: #666;
        }
        .input-area button {
            padding: 10px 20px;
            background: #333;
            color: #d4d4d4;
            border: 1px solid #444;
            border-radius: 4px;
            font-size: 14px;
            cursor: pointer;
            transition: background 0.2s;
            white-space: nowrap;
        }
        .input-area button:hover {
            background: #444;
        }
        .input-area button:disabled {
            opacity: 0.5;
            cursor: not-allowed;
        }
        .input-area .file-btn {
            background: #2a2a2a;
        }
        .input-area .file-btn:hover {
            background: #3a3a3a;
        }
        .empty-state {
            color: #666;
            text-align: center;
            padding: 60px 20px;
            font-size: 15px;
        }
        .status-bar {
            padding: 6px 24px;
            background: #1a1a1a;
            border-top: 1px solid #333;
            font-size: 12px;
            color: #666;
            display: flex;
            justify-content: space-between;
            flex-shrink: 0;
        }
        #fileInput {
            display: none;
        }
        @media (max-width: 700px) {
            .sidebar {
                width: 200px;
            }
            .messages .message {
                max-width: 85%;
            }
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>SecNet</h1>
        <div class="status">
            Connections: <span id="connCount">0</span>
            &nbsp;|&nbsp; Peer ID: <span id="myPeerId" style="font-family:monospace;font-size:12px;">loading...</span>
        </div>
    </div>
    <div class="container">
        <div class="sidebar">
            <div class="peer-id-box">
                <label>Your Peer ID</label>
                <input id="myPeerIdInput" readonly>
            </div>
            <div class="peer-list">
                <div class="title">Connected Peers</div>
                <div id="peerList"></div>
            </div>
        </div>
        <div class="main">
            <div class="messages" id="messages">
                <div class="empty-state">No messages yet.</div>
            </div>
            <div class="input-area">
                <input id="peerInput" placeholder="Peer ID">
                <input id="msgInput" placeholder="Message..." onkeydown="if(event.key==='Enter') sendMessage()">
                <button onclick="sendMessage()">Send</button>
                <button class="file-btn" onclick="document.getElementById('fileInput').click()">Send File</button>
                <input type="file" id="fileInput" onchange="sendFile(event)">
            </div>
            <div class="status-bar">
                <span id="statusText">Connected to global network</span>
                <span id="messageCount">0 messages</span>
            </div>
        </div>
    </div>

    <script>
        let myPeerId = '';

        function fetchStatus() {
            fetch('/api/status')
                .then(r => r.json())
                .then(data => {
                    myPeerId = data.peer_id;
                    document.getElementById('myPeerId').textContent = myPeerId.substring(0, 16) + '...';
                    document.getElementById('myPeerIdInput').value = myPeerId;
                })
                .catch(() => {});
        }

        function fetchPeers() {
            fetch('/api/list')
                .then(r => r.json())
                .then(data => {
                    document.getElementById('connCount').textContent = data.connections || 0;
                    const list = document.getElementById('peerList');
                    list.innerHTML = '';
                    if (!data.peers || data.peers.length === 0) {
                        list.innerHTML = '<div style="padding:16px;color:#666;font-size:13px;">No peers connected</div>';
                        return;
                    }
                    data.peers.forEach(id => {
                        const div = document.createElement('div');
                        div.className = 'peer-item';
                        div.innerHTML = '<span class="dot"></span><span class="id">' + id + '</span>';
                        div.onclick = function() {
                            document.getElementById('peerInput').value = id;
                        };
                        list.appendChild(div);
                    });
                })
                .catch(() => {});
        }

        function fetchMessages() {
            fetch('/api/messages')
                .then(r => r.json())
                .then(messages => {
                    const container = document.getElementById('messages');
                    container.innerHTML = '';
                    document.getElementById('messageCount').textContent = messages.length + ' messages';

                    if (!messages || messages.length === 0) {
                        container.innerHTML = '<div class="empty-state">No messages yet.</div>';
                        return;
                    }

                    messages.forEach(msg => {
                        const div = document.createElement('div');
                        const isIncoming = msg.from !== myPeerId;
                        div.className = 'message ' + (isIncoming ? 'incoming' : 'outgoing');

                        let statusHtml = '';
                        if (!isIncoming) {
                            statusHtml = msg.delivered ?
                                '<span class="delivered">✓ delivered</span>' :
                                '<span class="not-delivered">⏳ sending...</span>';
                        }

                        let content = msg.content;
                        if (msg.is_file) {
                            let downloadStatus = '';
                            if (msg.downloaded) {
                                downloadStatus = '<span class="file-downloaded">✓ downloaded</span>';
                            }
                            content = '📎 <span class="file-link" onclick="downloadFile(\'' + msg.file_id + '\', \'' + msg.file_name + '\')">' + msg.file_name + '</span> ' + downloadStatus;
                        }

                        div.innerHTML = content +
                            '<div class="meta">' +
                            (isIncoming ? 'From: ' + msg.from.substring(0, 12) + '...' : 'You') +
                            ' at ' + msg.time +
                            statusHtml +
                            '</div>';
                        container.appendChild(div);
                    });
                    container.scrollTop = container.scrollHeight;
                })
                .catch(() => {});
        }

        function downloadFile(fileId, fileName) {
            fetch('/api/download?id=' + fileId)
                .then(r => r.blob())
                .then(blob => {
                    const url = URL.createObjectURL(blob);
                    const a = document.createElement('a');
                    a.href = url;
                    a.download = fileName;
                    document.body.appendChild(a);
                    a.click();
                    document.body.removeChild(a);
                    URL.revokeObjectURL(url);
                    fetchMessages();
                })
                .catch(err => alert('Download error: ' + err));
        }

        function sendMessage() {
            const peerInput = document.getElementById('peerInput');
            const msgInput = document.getElementById('msgInput');
            const peerId = peerInput.value.trim();
            const msg = msgInput.value.trim();

            if (!peerId || !msg) {
                alert('Enter Peer ID and message');
                return;
            }

            const btn = document.querySelector('.input-area button:not(.file-btn)');
            btn.disabled = true;

            fetch('/api/send', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ peer_id: peerId, message: msg })
            })
            .then(r => r.json())
            .then(() => {
                msgInput.value = '';
                fetchMessages();
            })
            .catch(err => alert('Send error: ' + err))
            .finally(() => btn.disabled = false);
        }

        function sendFile(event) {
            const file = event.target.files[0];
            if (!file) return;

            const peerId = document.getElementById('peerInput').value.trim();
            if (!peerId) {
                alert('Enter Peer ID first');
                event.target.value = '';
                return;
            }

            const formData = new FormData();
            formData.append('peer_id', peerId);
            formData.append('file', file);

            fetch('/api/sendfile', {
                method: 'POST',
                body: formData
            })
            .then(r => r.json())
            .then(() => {
                fetchMessages();
                event.target.value = '';
            })
            .catch(err => alert('File send error: ' + err));
        }

        fetchStatus();
        fetchPeers();
        fetchMessages();
        setInterval(fetchPeers, 3000);
        setInterval(fetchMessages, 2000);
        setInterval(fetchStatus, 10000);
    </script>
</body>
</html>`