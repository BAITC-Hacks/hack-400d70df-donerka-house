// ===================== CHAT.JS — EKT.KZ AI Assistant =====================

const API_CHAT = '/api/chat';
let isOpen = false;
let isBusy = false;
let chipsHidden = false;

function toggleChat() {
  const widget = document.getElementById('chatWidget');
  isOpen = !isOpen;
  if (isOpen) {
    widget.classList.add('open');
    scrollBottom();
    setTimeout(() => document.getElementById('chatInput').focus(), 100);
  } else {
    widget.classList.remove('open');
  }
}

function openChat() {
  if (!isOpen) toggleChat();
}

function sendChip(btn) {
  hideChips();
  sendText(btn.textContent.trim());
}

function sendMsg() {
  const inp = document.getElementById('chatInput');
  const text = inp.value.trim();
  if (!text || isBusy) return;
  inp.value = '';
  hideChips();
  sendText(text);
}

function hideChips() {
  if (!chipsHidden) {
    document.getElementById('chatChips').style.display = 'none';
    chipsHidden = true;
  }
}

async function sendText(text) {
  if (isBusy) return;
  isBusy = true;
  document.getElementById('chatSendBtn').disabled = true;

  appendMsg(text, 'user');
  const typingId = showTyping();
  scrollBottom();

  try {
    const res = await fetch(API_CHAT, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ message: text }),
    });

    removeTyping(typingId);

    if (!res.ok) {
      const err = await res.json().catch(() => ({}));
      appendMsg('⚠️ ' + (err.error || 'Ошибка. Попробуйте снова.'), 'bot');
    } else {
      const data = await res.json();
      appendMsg(data.reply || '...', 'bot');
    }
  } catch {
    removeTyping(typingId);
    appendMsg('⚠️ Нет соединения с сервером.', 'bot');
  } finally {
    isBusy = false;
    document.getElementById('chatSendBtn').disabled = false;
    scrollBottom();
    document.getElementById('chatInput').focus();
  }
}

function appendMsg(text, role) {
  const container = document.getElementById('chatMessages');
  const wrap = document.createElement('div');
  wrap.className = `msg ${role}`;

  const bubble = document.createElement('div');
  bubble.className = 'msg-bubble';
  bubble.innerHTML = text.replace(/\n/g, '<br/>');

  const time = document.createElement('div');
  time.className = 'msg-time';
  time.textContent = now();

  wrap.appendChild(bubble);
  wrap.appendChild(time);
  container.appendChild(wrap);
  scrollBottom();
}

function showTyping() {
  const id = 'typ-' + Date.now();
  const container = document.getElementById('chatMessages');
  const wrap = document.createElement('div');
  wrap.id = id;
  wrap.className = 'msg bot typing';
  wrap.innerHTML = `<div class="msg-bubble"><span class="dot"></span><span class="dot"></span><span class="dot"></span></div>`;
  container.appendChild(wrap);
  scrollBottom();
  return id;
}

function removeTyping(id) {
  const el = document.getElementById(id);
  if (el) el.remove();
}

function scrollBottom() {
  const c = document.getElementById('chatMessages');
  setTimeout(() => { c.scrollTop = c.scrollHeight; }, 50);
}

function now() {
  const d = new Date();
  return d.getHours().toString().padStart(2,'0') + ':' + d.getMinutes().toString().padStart(2,'0');
}

// Close on outside click
document.addEventListener('click', e => {
  const widget = document.getElementById('chatWidget');
  const bubble = document.getElementById('chatBubble');
  if (isOpen && !widget.contains(e.target) && !bubble.contains(e.target)) {
    toggleChat();
  }
});
