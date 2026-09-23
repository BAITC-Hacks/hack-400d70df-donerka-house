// ===================== CHAT.JS — EKT.KZ AI Assistant =====================

const API_CHAT = '/api/chat';
const API_CART = '/api/cart';
const API_AUTH = '/api/auth';
let isOpen = false;
let isBusy = false;
let chipsHidden = false;
let cart = [];
let authMode = 'login';

let sessionPromise;
async function sessionToken() {
 if (!sessionPromise) sessionPromise = fetch('/api/session').then(async res => {
  if (!res.ok) throw new Error('Session unavailable');
  return (await res.json()).csrf_token;
 }).catch(error => { sessionPromise = null; throw error; });
 return sessionPromise;
}
async function apiFetch(url, options = {}) {
 const token = await sessionToken();
 const method = options.method || 'GET';
 const headers = {...options.headers};
 if (!['GET', 'HEAD'].includes(method)) {
  headers['X-CSRF-Token'] = token;
  headers['Content-Type'] = 'application/json';
  if (!options.body) options.body = '{}';
 }
 return fetch(url, {...options, headers, credentials: 'same-origin'});
}

const paymentDataPattern = /(?:\d[ \t-]*){13,19}|\b[A-Z]{2}\d{2}[A-Z0-9]{11,30}\b|(?:cvv|cvc|пин|pin|otp|смс|sms|код.{0,12}(?:банк|подтверж)|срок.{0,12}(?:карт|действ))\s*[:=-]?\s*\d{2,8}/i;
const paymentPrivacyReply = 'Не отправляйте номер карты, срок действия, CVV/CVC, PIN и коды SMS. Сообщение с распознаваемыми платёжными данными не отправлено. Оплата — только на защищённой странице EKT.';

function safeURL(value) {
  try {
    const url = new URL(value || '#', window.location.origin);
    return ['http:', 'https:'].includes(url.protocol) ? url.href : '#';
  } catch {
    return '#';
  }
}

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

async function sendText(text, confirmationToken = '') {
 if (paymentDataPattern.test(text)) {
  document.getElementById('chatInput').value = '';
  appendMsg(paymentPrivacyReply, 'bot'); scrollBottom(); return;
 }
  if (isBusy) return;
  isBusy = true;
  document.getElementById('chatSendBtn').disabled = true;

  appendMsg(text, 'user');
  const typingId = showTyping();
  scrollBottom();

  try {
    const res = await apiFetch(API_CHAT, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ message: text, confirmation_token: confirmationToken }),
    });

    removeTyping(typingId);

    if (!res.ok) {
      const err = await res.json().catch(() => ({}));
      appendMsg('⚠️ ' + (err.error || 'Ошибка. Попробуйте снова.'), 'bot');
    } else {
      const data = await res.json();
      appendMsg(data.reply || '...', 'bot');
      document.querySelectorAll('.confirm-add-btn').forEach(button => { button.disabled = true; });
      if (data.confirmation_token) {
       const button = document.createElement('button');
       button.className = 'chat-add-btn confirm-add-btn';
       button.textContent = 'Подтверждаю добавление';
       button.addEventListener('click', () => { if (!isBusy) sendText('да, добавь', data.confirmation_token); });
       document.getElementById('chatMessages').appendChild(button);
      }

      if (data.products && data.products.length > 0) {
        appendProductCards(data.products);
      }

      if (data.analogs && data.analogs.length > 0) {
        appendProductCards(data.analogs, 'Подходящие аналоги');
      }

      if (data.cart_url) {
        appendCartLink(data.cart_url);
      }

      if (data.cart) {
        applyCart(data.cart);
      } else if (data.cart_action) {
        // Backward-compatible fallback for an older backend response.
        await loadCart();
      }
      if (data.cart_action) {
        showToast(`✅ ${data.cart_action.name} (x${data.cart_action.quantity}) добавлен в корзину!`);
      }
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

// ==== CART MANAGEMENT ====

async function loadCart() {
  try {
    const res = await apiFetch(API_CART);
    if (res.ok) applyCart(await res.json());
  } catch {
    showToast('⚠️ Не удалось загрузить корзину');
  }
}

// ==== LOCAL ACCOUNT MANAGEMENT ====

async function loadAuth() {
  try {
    const res = await apiFetch(API_AUTH);
    if (res.ok) applyAuthState(await res.json());
  } catch {
    // The basket remains usable as a guest cart if the account endpoint is unavailable.
  }
}

function openAuthModal() {
  const modal = document.getElementById('authModal');
  if (!modal) return;
  modal.classList.add('open');
  modal.setAttribute('aria-hidden', 'false');
  setTimeout(() => {
    const field = document.getElementById('authEmail');
    if (field) field.focus();
  }, 50);
}

function closeAuthModal() {
  const modal = document.getElementById('authModal');
  if (!modal) return;
  modal.classList.remove('open');
  modal.setAttribute('aria-hidden', 'true');
  const error = document.getElementById('authError');
  if (error) error.textContent = '';
}

function toggleAuthMode() {
  authMode = authMode === 'login' ? 'register' : 'login';
  const register = authMode === 'register';
  document.getElementById('authTitle').textContent = register ? 'Создать аккаунт' : 'Вход в аккаунт';
  document.getElementById('authNameField').style.display = register ? 'block' : 'none';
  document.getElementById('authPassword').setAttribute('autocomplete', register ? 'new-password' : 'current-password');
  document.getElementById('authSubmit').textContent = register ? 'Зарегистрироваться' : 'Войти';
  document.getElementById('authModeToggle').textContent = register ? 'У меня уже есть аккаунт' : 'Создать новый аккаунт';
  document.getElementById('authError').textContent = '';
}

async function submitAuth(event) {
  event.preventDefault();
  if (isBusy) { showToast('Дождитесь ответа перед сменой аккаунта.'); return; }
  const submit = document.getElementById('authSubmit');
  const error = document.getElementById('authError');
  const payload = {
    action: authMode,
    email: document.getElementById('authEmail').value.trim(),
    password: document.getElementById('authPassword').value,
    name: document.getElementById('authName').value.trim(),
    merge_cart: document.getElementById('mergeCartConsent').checked,
  };
  error.textContent = '';
  submit.disabled = true;
  try {
    const res = await apiFetch(API_AUTH, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      error.textContent = data.error || 'Не удалось выполнить вход';
      return;
    }
    applyAuthState(data);
    resetChatView();
    document.getElementById('authPassword').value = '';
    closeAuthModal();
    await loadCart();
    showToast(`✅ Добро пожаловать, ${data.user?.name || data.user?.email || 'пользователь'}!`);
  } catch {
    error.textContent = 'Нет соединения с сервером';
  } finally {
    submit.disabled = false;
  }
}

async function logoutAccount() {
  if (isBusy) { showToast('Дождитесь ответа перед выходом.'); return; }
  try {
    const res = await apiFetch(API_AUTH, { method: 'DELETE' });
    if (!res.ok) throw new Error('logout failed');
    applyAuthState({ authenticated: false });
    resetChatView();
    closeAuthModal();
    await loadCart();
    showToast('Вы вышли из аккаунта');
  } catch {
    showToast('⚠️ Не удалось выйти из аккаунта');
  }
}

function applyAuthState(data) {
  const authenticated = Boolean(data && data.authenticated && data.user);
  const accountButton = document.getElementById('accountButton');
  const guest = document.getElementById('authGuestView');
  const user = document.getElementById('authUserView');
  const note = document.getElementById('cartNote');
  if (accountButton) accountButton.textContent = authenticated ? `👤 ${data.user.name || data.user.email}` : 'Войти';
  if (guest) guest.style.display = authenticated ? 'none' : 'block';
  if (user) user.style.display = authenticated ? 'block' : 'none';
  if (authenticated) {
    document.getElementById('authUserName').textContent = data.user.name || 'Аккаунт';
    document.getElementById('authUserEmail').textContent = data.user.email || '';
    if (note) note.textContent = 'Корзина сохранена за вашим аккаунтом.';
  } else if (note) {
    note.textContent = 'Корзина сохранена для этой сессии браузера. Войдите, чтобы сохранить её за аккаунтом.';
  }
}

function applyCart(data) {
  cart = Array.isArray(data.items) ? data.items : [];
  updateCartUI();
  renderCartPanel();
}

async function confirmCartRemoval(action, article = '') {
 try {
  const proposal = await apiFetch('/api/cart/confirmation', {
   method: 'POST', body: JSON.stringify({action, article})
  });
  const data = await proposal.json();
  if (!proposal.ok) throw new Error(data.error || 'Не удалось подготовить изменение');
  if (!window.confirm(data.summary)) return;
  const response = await apiFetch(API_CART, {
   method: 'DELETE', body: JSON.stringify({confirmation_token: data.confirmation_token})
  });
  const result = await response.json();
  if (!response.ok) throw new Error(result.error || 'Корзина изменилась; подтвердите действие заново');
  applyCart(result);
 } catch (error) { showToast('⚠️ ' + error.message); }
}
async function removeCartItem(article) { await confirmCartRemoval('remove', article); }
async function clearCart() { await confirmCartRemoval('clear'); }

async function clearChat() {
 if (isBusy) { showToast('Дождитесь ответа, затем очистите историю.'); return; }
 if (!window.confirm('Удалить историю чата и отменить ожидающее подтверждение?')) return;
 try {
  const response = await apiFetch(API_CHAT, {method:'DELETE'});
  if (!response.ok) throw new Error('Не удалось очистить историю');
  resetChatView();
 } catch (error) { showToast(error.message); }
}
function resetChatView() {
 document.getElementById('chatMessages').textContent = '';
 appendMsg('Помогу выбрать товар. Не отправляйте платёжные данные. Изменения корзины требуют вашего подтверждения.', 'bot');
}

function toggleCart() {
  const panel = document.getElementById('cartPanel');
  if (panel) panel.classList.toggle('open');
}

function closeCart() {
  const panel = document.getElementById('cartPanel');
  if (panel) panel.classList.remove('open');
}

function updateCartUI() {
  const count = cart.reduce((acc, item) => acc + item.quantity, 0);
  const total = cart.reduce((acc, item) => acc + (item.price * item.quantity), 0);
  const countEl = document.getElementById('cartCount');
  const totalEl = document.getElementById('cartTotal');
  const panelTotalEl = document.getElementById('cartPanelTotal');

  if (countEl) countEl.textContent = count;
  if (totalEl) totalEl.textContent = total.toLocaleString('ru-KZ') + ' тг';
  if (panelTotalEl) panelTotalEl.textContent = total.toLocaleString('ru-KZ') + ' тг';

  if (countEl) {
    countEl.style.transform = 'scale(1.5)';
    setTimeout(() => { countEl.style.transform = 'scale(1)'; }, 200);
  }
}

function renderCartPanel() {
  const container = document.getElementById('cartItems');
  const clearButton = document.getElementById('clearCartBtn');
  if (!container) return;
  container.textContent = '';
  if (clearButton) clearButton.disabled = cart.length === 0;

  if (cart.length === 0) {
    const empty = document.createElement('p');
    empty.className = 'cart-empty';
    empty.textContent = 'Корзина пока пуста';
    container.appendChild(empty);
    return;
  }

  cart.forEach(item => {
    const row = document.createElement('div');
    row.className = 'cart-item';

    const details = document.createElement('div');
    details.className = 'cart-item-details';
    const name = document.createElement('div');
    name.className = 'cart-item-name';
    name.textContent = item.name;
    const meta = document.createElement('div');
    meta.className = 'cart-item-meta';
    meta.textContent = `Арт. ${item.article} · ${item.quantity} шт.`;
    details.append(name, meta);

    const controls = document.createElement('div');
    controls.className = 'cart-item-controls';
    const price = document.createElement('span');
    price.textContent = (item.price * item.quantity).toLocaleString('ru-KZ') + ' тг';
    const remove = document.createElement('button');
    remove.className = 'cart-remove-btn';
    remove.type = 'button';
    remove.title = 'Удалить товар';
    remove.textContent = '×';
    remove.addEventListener('click', () => removeCartItem(item.article));
    controls.append(price, remove);

    row.append(details, controls);
    container.appendChild(row);
  });
}

function showToast(msg) {
  const container = document.getElementById('toastContainer');
  if (!container) return;
  const toast = document.createElement('div');
  toast.className = 'toast';
  toast.textContent = msg;
  container.appendChild(toast);
  setTimeout(() => {
    toast.classList.add('hide');
    setTimeout(() => toast.remove(), 300);
  }, 4000);
}

function appendMsg(text, role) {
  const container = document.getElementById('chatMessages');
  const wrap = document.createElement('div');
  wrap.className = `msg ${role}`;
  const bubble = document.createElement('div');
  bubble.className = 'msg-bubble';
  bubble.textContent = text;
  const time = document.createElement('div');
  time.className = 'msg-time';
  time.textContent = now();
  wrap.append(bubble, time);
  container.appendChild(wrap);
  scrollBottom();
}

function appendCartLink(url) {
  const container = document.getElementById('chatMessages');
  const wrap = document.createElement('div');
  wrap.className = 'msg bot';
  const link = document.createElement('a');
  link.className = 'chat-product-link';
  link.href = safeURL(url);
  link.textContent = '🛒 Открыть актуальную корзину';
  link.addEventListener('click', event => {
    if (url === '/#cart' || url === '#cart') {
      event.preventDefault();
      history.replaceState(null, '', '#cart');
      const panel = document.getElementById('cartPanel');
      if (panel) panel.classList.add('open');
    }
  });
  wrap.appendChild(link);
  container.appendChild(wrap);
  scrollBottom();
}

function appendProductCards(products, title = '') {
  const container = document.getElementById('chatMessages');
  const wrap = document.createElement('div');
  wrap.className = 'msg bot';
  if (title) {
    const heading = document.createElement('div');
    heading.className = 'chat-product-section-title';
    heading.textContent = title;
    wrap.appendChild(heading);
  }

  const cardsWrap = document.createElement('div');
  cardsWrap.className = 'product-cards-row';

  products.forEach(product => {
    const card = document.createElement('div');
    card.className = 'chat-product-card';

    const imgWrap = document.createElement('div');
    imgWrap.className = 'chat-product-img-wrap';
    const placeholder = document.createElement('div');
    placeholder.className = 'chat-product-placeholder';
    placeholder.textContent = '⚡';
    imgWrap.appendChild(placeholder);
    if (product.image && safeURL(product.image) !== '#') {
      const img = document.createElement('img');
      img.src = safeURL(product.image);
      img.alt = product.name || 'Товар';
      img.className = 'chat-product-img';
      img.onerror = () => { img.remove(); };
      imgWrap.insertBefore(img, placeholder);
      placeholder.style.display = 'none';
    }

    const info = document.createElement('div');
    info.className = 'chat-product-info';
    const article = document.createElement('div');
    article.className = 'chat-product-article';
    article.textContent = 'Арт: ' + (product.article || '—');
    const name = document.createElement('div');
    name.className = 'chat-product-name';
    name.textContent = product.name || 'Товар';
    const verified = product.verified_at && Date.now() - Date.parse(product.verified_at) < 5 * 60 * 1000;
    if (verified && product.availability) {
      const stock = document.createElement('div');
      stock.className = 'chat-product-stock ' + (product.availability === 'В наличии' ? 'is-available' : 'is-preorder');
      stock.textContent = product.availability;
      if (product.stock_quantity > 0) {
        stock.textContent += ` · до ${product.stock_quantity} шт.`;
      } else if (product.total_stock_quantity > 0) {
        stock.textContent += ` · всего ${product.total_stock_quantity} шт.`;
      }
      if (product.stock_location) {
        stock.textContent += ` · ${product.stock_location}`;
      }
      info.appendChild(stock);
    }
    const footer = document.createElement('div');
    footer.className = 'chat-product-footer';
    const price = document.createElement('span');
    price.className = 'chat-product-price';
    price.textContent = verified && product.price ? product.price.toLocaleString('ru-KZ') + ' тг' : 'Цена требует проверки';
    const link = document.createElement('a');
    link.className = 'chat-product-link';
    link.textContent = 'Подробнее →';
    link.href = safeURL(product.url);
    link.target = '_blank';
    link.rel = 'noopener noreferrer';
    const addButton = document.createElement('button');
    addButton.type = 'button';
    addButton.className = 'chat-add-btn';
    addButton.textContent = 'В корзину';
    addButton.addEventListener('click', () => {
      openChat();
      sendText(`добавь 1 шт ${product.article}`);
    });

    footer.append(price, link, addButton);
    info.append(article, name);
    const provenance = document.createElement('div');
    provenance.className = 'chat-product-meta';
    provenance.textContent = verified ? 'Данные EKT проверены ' + new Date(product.verified_at).toLocaleTimeString('ru-RU') : 'Актуальное наличие требует проверки';
    info.appendChild(provenance);
    if (product.recommendation_reason) {
     const reason = document.createElement('p');
     reason.className = 'chat-product-meta';
     reason.textContent = 'Почему предложен: ' + product.recommendation_reason;
     info.appendChild(reason);
    }

    if (Array.isArray(product.certificates) && product.certificates.length > 0) {
      const certs = document.createElement('div');
      certs.className = 'chat-product-certificates';
      product.certificates.forEach((certificate, index) => {
        const certLink = document.createElement('a');
        certLink.href = safeURL(certificate);
        certLink.target = '_blank';
        certLink.rel = 'noopener noreferrer';
        certLink.textContent = index === 0 ? '📄 Сертификат' : `📄 Сертификат ${index + 1}`;
        certs.appendChild(certLink);
      });
      info.appendChild(certs);
    }

    info.appendChild(footer);
    card.append(imgWrap, info);
    cardsWrap.appendChild(card);
  });

  wrap.appendChild(cardsWrap);
  container.appendChild(wrap);
  scrollBottom();
}

function showTyping() {
  const id = 'typ-' + Date.now();
  const container = document.getElementById('chatMessages');
  const wrap = document.createElement('div');
  wrap.id = id;
  wrap.className = 'msg bot typing';
  wrap.innerHTML = '<div class="msg-bubble"><span class="dot"></span><span class="dot"></span><span class="dot"></span></div>';
  container.appendChild(wrap);
  scrollBottom();
  return id;
}

function removeTyping(id) {
  const el = document.getElementById(id);
  if (el) el.remove();
}

function scrollBottom() {
  const container = document.getElementById('chatMessages');
  if (container) setTimeout(() => { container.scrollTop = container.scrollHeight; }, 60);
}

function now() {
  const date = new Date();
  return date.getHours().toString().padStart(2, '0') + ':' + date.getMinutes().toString().padStart(2, '0');
}

document.addEventListener('DOMContentLoaded', async () => {
  document.getElementById('cartButton').addEventListener('keydown', event => {
    if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); toggleCart(); }
  });
  await loadAuth();
  await loadCart();
  if (window.location.hash === '#cart') {
    const panel = document.getElementById('cartPanel');
    if (panel) panel.classList.add('open');
  }
});

document.addEventListener('click', event => {
  const widget = document.getElementById('chatWidget');
  const bubble = document.getElementById('chatBubble');
  const cartPanel = document.getElementById('cartPanel');
  const cartButton = document.getElementById('cartButton');
  const authModal = document.getElementById('authModal');
  if (isOpen && widget && bubble && !widget.contains(event.target) && !bubble.contains(event.target) && !event.target.closest('[data-chat-trigger]')) {
    toggleChat();
  }
  if (cartPanel && cartPanel.classList.contains('open') && !cartPanel.contains(event.target) && !cartButton.contains(event.target)) {
    closeCart();
  }
  if (authModal && authModal.classList.contains('open') && event.target === authModal) {
    closeAuthModal();
  }
});
