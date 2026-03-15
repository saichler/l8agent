/*
(c) 2025 Sharon Aicler (saichler@gmail.com)

Layer 8 Ecosystem is licensed under the Apache License, Version 2.0.
*/
// l8agent Mobile Chat UI Component
// Uses Layer8MAuth for HTTP calls, Layer8MConfig for endpoint resolution,
// Layer8MTable for inline data results, --layer8d-* theme vars.

(function() {
    'use strict';

    window.L8AgentChatMobile = {
        _config: null,
        _container: null,
        _conversationId: null,
        _messages: [],
        _loading: false,

        init: function(config) {
            this._config = config;
            this._container = document.getElementById(config.containerId);
            if (!this._container) return;
            this._render();
            this._attachEvents();
        },

        _render: function() {
            this._container.innerHTML =
                '<div class="l8agent-m-chat">' +
                    '<div class="l8agent-m-header">' +
                        '<div class="l8agent-m-title">AI Assistant</div>' +
                        '<button class="l8agent-m-new-btn">New</button>' +
                    '</div>' +
                    '<div class="l8agent-m-messages"></div>' +
                    '<div class="l8agent-m-input-area">' +
                        '<textarea class="l8agent-m-input" placeholder="Ask a question..." rows="2"></textarea>' +
                        '<button class="l8agent-m-send-btn">Send</button>' +
                    '</div>' +
                '</div>';
        },

        _attachEvents: function() {
            var self = this;
            var sendBtn = this._container.querySelector('.l8agent-m-send-btn');
            var input = this._container.querySelector('.l8agent-m-input');
            var newBtn = this._container.querySelector('.l8agent-m-new-btn');

            sendBtn.addEventListener('click', function() { self._send(); });
            input.addEventListener('keydown', function(e) {
                if (e.key === 'Enter' && !e.shiftKey) {
                    e.preventDefault();
                    self._send();
                }
            });
            newBtn.addEventListener('click', function() { self.newConversation(); });
        },

        _send: function() {
            if (this._loading) return;
            var input = this._container.querySelector('.l8agent-m-input');
            var text = input.value.trim();
            if (!text) return;

            input.value = '';
            this._addMessage('user', text);
            this.sendMessage(text);
        },

        sendMessage: function(text) {
            if (this._loading) return;
            var self = this;
            this._loading = true;
            this._showLoading();

            var body = { message: text };
            if (this._conversationId) body.conversationId = this._conversationId;

            var endpoint = Layer8MConfig.resolveEndpoint(this._config.chatEndpoint);
            Layer8MAuth.post(endpoint, body)
            .then(function(data) {
                self._loading = false;
                self._hideLoading();
                if (data.error) {
                    self._addMessage('assistant', 'Error: ' + data.error);
                    return;
                }
                if (data.conversationId) {
                    self._conversationId = data.conversationId;
                }
                self._addMessage('assistant', data.response || 'No response');
                if (data.dataResults && data.dataResults.length > 0) {
                    self._renderDataResults(data.dataResults);
                }
                self._renderTokenInfo(data.toolCallsMade, data.totalTokens);
            })
            .catch(function(err) {
                self._loading = false;
                self._hideLoading();
                self._addMessage('assistant', 'Error: ' + (err.message || err));
            });
        },

        loadConversation: function(id) {
            var self = this;
            var query = 'select * from L8AgentConversation where conversationId=' + id;
            var body = encodeURIComponent(JSON.stringify({ text: query }));
            var endpoint = Layer8MConfig.resolveEndpoint(this._config.convoEndpoint) + '?body=' + body;

            Layer8MAuth.get(endpoint)
            .then(function(data) {
                var convo = null;
                if (data.list && data.list.length > 0) convo = data.list[0];
                if (!convo) return;
                self._conversationId = convo.conversationId;
                self._messages = [];
                var msgArea = self._container.querySelector('.l8agent-m-messages');
                msgArea.innerHTML = '';
                if (convo.messages) {
                    convo.messages.forEach(function(msg) {
                        var role = msg.role === 2 ? 'assistant' : 'user';
                        self._addMessage(role, msg.content);
                    });
                }
            });
        },

        newConversation: function() {
            this._conversationId = null;
            this._messages = [];
            var msgArea = this._container.querySelector('.l8agent-m-messages');
            if (msgArea) msgArea.innerHTML = '';
        },

        _addMessage: function(role, content) {
            this._messages.push({ role: role, content: content });
            var msgArea = this._container.querySelector('.l8agent-m-messages');
            var div = document.createElement('div');
            div.className = 'l8agent-m-msg l8agent-m-msg-' + role;

            var label = document.createElement('div');
            label.className = 'l8agent-m-msg-role';
            label.textContent = role === 'user' ? 'You' : 'AI Assistant';

            var body = document.createElement('div');
            body.className = 'l8agent-m-msg-body';
            body.textContent = content;

            div.appendChild(label);
            div.appendChild(body);
            msgArea.appendChild(div);
            msgArea.scrollTop = msgArea.scrollHeight;
        },

        _renderDataResults: function(results) {
            var msgArea = this._container.querySelector('.l8agent-m-messages');
            results.forEach(function(result) {
                if (!result.dataJson) return;
                var div = document.createElement('div');
                div.className = 'l8agent-m-data-result';
                var title = document.createElement('div');
                title.className = 'l8agent-m-data-title';
                title.textContent = 'Query: ' + (result.query || result.modelName);
                div.appendChild(title);

                var tableDiv = document.createElement('div');
                tableDiv.className = 'l8agent-m-data-table';
                var tableId = 'l8agent-m-result-' + Date.now() + '-' + Math.random().toString(36).substr(2, 5);
                tableDiv.id = tableId;
                div.appendChild(tableDiv);
                msgArea.appendChild(div);

                try {
                    var items = JSON.parse(result.dataJson);
                    if (Array.isArray(items) && items.length > 0) {
                        var cols = Object.keys(items[0]).map(function(key) {
                            return { key: key, label: key };
                        });
                        var table = new Layer8MTable(tableId, {
                            columns: cols,
                            rowsPerPage: 5
                        });
                        table.setData(items);
                    }
                } catch (e) {
                    tableDiv.textContent = result.dataJson;
                }
            });
            msgArea.scrollTop = msgArea.scrollHeight;
        },

        _renderTokenInfo: function(toolCalls, tokens) {
            if (!toolCalls && !tokens) return;
            var msgArea = this._container.querySelector('.l8agent-m-messages');
            var info = document.createElement('div');
            info.className = 'l8agent-m-token-info';
            var parts = [];
            if (toolCalls) parts.push(toolCalls + ' tool call' + (toolCalls > 1 ? 's' : ''));
            if (tokens) parts.push(tokens + ' tokens');
            info.textContent = parts.join(' | ');
            msgArea.appendChild(info);
        },

        _showLoading: function() {
            var msgArea = this._container.querySelector('.l8agent-m-messages');
            var div = document.createElement('div');
            div.className = 'l8agent-m-loading';
            div.id = 'l8agent-m-loading-indicator';
            div.innerHTML = '<span class="l8agent-m-dot"></span><span class="l8agent-m-dot"></span><span class="l8agent-m-dot"></span>';
            msgArea.appendChild(div);
            msgArea.scrollTop = msgArea.scrollHeight;
        },

        _hideLoading: function() {
            var el = document.getElementById('l8agent-m-loading-indicator');
            if (el) el.remove();
        }
    };
})();
