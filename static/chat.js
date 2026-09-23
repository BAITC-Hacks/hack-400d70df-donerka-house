// ====================== CHAT.JS ======================
// Donerka House AI Chat Widget

const API_URL = '/api/chat';

let isOpen = false;
let isWaiting = false;
let suggestionsHidden = false;

// Toggle chat open/close
function toggleChat() {
  const widget = document.getElementById('chatWidget');
  const badge  = document.getElementById('chatBubbleBadge');
  isOpen = !isOpen;

  if (isOpen) {
    widget.classList.add('open');
    badge.style.display = 'none';
    scrollToBottom();
    document.getElementById('chatInput').focus();
  } else {
    widget.classList.remove('open');
  }
}

// Open chat programmatically (from hero button)
function openChat() {
  if (!isOpen) toggleChat();
}

// Send message on Enter key
function handleKeyPress(e) {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault();
    sendMessage();
  }
}

// Send a suggestion chip
function sendSuggestion(btn) {
  const text = btn.textContent.trim();
  // Hide suggestions after first use
  if (!suggestionsHidden) {
    document.getElementById('chatSuggestions').style.display = 'none';
    suggestionsHidden = true;
  }
  sendMessageText(text);
}

// Send current input value
function sendMessage() {
  const input = document.getElementById('chatInput');
  const text = input.value.trim();
  if (!text || isWaiting) return;
  input.value = '';

  if (!suggestionsHidden) {
    document.getElementById('chatSuggestions').style.display = 'none';
    suggestionsHidden = true;
  }

  sendMessageText(text);
}

// Core: append user message, show typing, call API, show reply
async function sendMessageText(text) {
  if (isWaiting) return;
  isWaiting = true;

  // Disable send button
  const sendBtn = document.getElementById('chatSendBtn');
  sendBtn.disabled = true;

  // Append user message
  appendMessage(text, 'user');

  // Show typing indicator
  const typingId = showTyping();
  scrollToBottom();

  try {
    const res = await fetch(API_URL, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ message: text }),
    });

    removeTyping(typingId);

    if (!res.ok) {
      const err = await res.json().catch(() => ({ error: 'Ошибка сервера' }));
      appendMessage('⚠️ ' + (err.error || 'Произошла ошибка. Попробуйте позже.'), 'bot');
    } else {
      const data = await res.json();
      appendMessage(data.reply || '...', 'bot');
    }
  } catch (e) {
    removeTyping(typingId);
    appendMessage('⚠️ Не удалось подключиться к серверу. Проверьте соединение.', 'bot');
  } finally {
    isWaiting = false;
    sendBtn.disabled = false;
    scrollToBottom();
    document.getElementById('chatInput').focus();
  }
}

// Append a message bubble to the chat
function appendMessage(text, sender) {
  const container = document.getElementById('chatMessages');
  const div = document.createElement('div');
  div.className = `message ${sender === 'bot' ? 'bot-message' : 'user-message'}`;

  const bubble = document.createElement('div');
  bubble.className = 'message-bubble';
  // Support simple line breaks
  bubble.innerHTML = text.replace(/\n/g, '<br/>');

  const time = document.createElement('div');
  time.className = 'message-time';
  time.textContent = getTimeString();

  div.appendChild(bubble);
  div.appendChild(time);
  container.appendChild(div);
  scrollToBottom();
}

// Show animated typing indicator, returns its id
function showTyping() {
  const container = document.getElementById('chatMessages');
  const id = 'typing-' + Date.now();
  const div = document.createElement('div');
  div.id = id;
  div.className = 'message bot-message typing-indicator';
  div.innerHTML = `<div class="message-bubble">
    <span class="typing-dot"></span>
    <span class="typing-dot"></span>
    <span class="typing-dot"></span>
  </div>`;
  container.appendChild(div);
  scrollToBottom();
  return id;
}

// Remove typing indicator by id
function removeTyping(id) {
  const el = document.getElementById(id);
  if (el) el.remove();
}

// Scroll messages to bottom
function scrollToBottom() {
  const container = document.getElementById('chatMessages');
  setTimeout(() => { container.scrollTop = container.scrollHeight; }, 50);
}

// Format current time as HH:MM
function getTimeString() {
  const now = new Date();
  return now.getHours().toString().padStart(2, '0') + ':' +
         now.getMinutes().toString().padStart(2, '0');
}

// Show new message badge on bubble when chat is closed
function notifyBadge() {
  if (!isOpen) {
    document.getElementById('chatBubbleBadge').style.display = 'flex';
  }
}

// Close chat on outside click
document.addEventListener('click', function(e) {
  const widget = document.getElementById('chatWidget');
  const bubble = document.getElementById('chatBubble');
  if (isOpen && !widget.contains(e.target) && !bubble.contains(e.target)) {
    toggleChat();
  }
});
