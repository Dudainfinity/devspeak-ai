// DevSpeak AI — frontend completo: auth, simulador adaptativo, mic, dashboard.

const API_BASE = (location.protocol === 'file:') ? 'http://localhost:8080' : '';
const TOKEN_KEY  = 'devspeak.token.v1';
const PROFILE_KEY_LEGACY = 'devspeak.profile.v1'; // fallback p/ versão sem login

let token = null;
let user  = null;
let profile = null;       // perfil ativo (vem do user logado, ou legacy localStorage)
let questions = [];
let currentIdx = 0;

const FALLBACK_QUESTIONS = [
  { id: 'fb-1', text: 'Tell me about a project you are proud of. What was your role and what would you do differently today?', topic: 'experience', stack: 'any', level: 'any' },
  { id: 'fb-2', text: 'Walk me through how you debug a problem you have never seen before.', topic: 'problem solving', stack: 'any', level: 'any' },
  { id: 'fb-3', text: 'Describe a technical disagreement with a teammate. How did you resolve it?', topic: 'collaboration', stack: 'any', level: 'any' },
];

// ─── Fetch autenticado ──────────────────────────────────────────────────────
async function apiFetch(path, opts = {}) {
  const headers = { ...(opts.headers || {}) };
  if (opts.body && !headers['Content-Type']) headers['Content-Type'] = 'application/json';
  if (token) headers['Authorization'] = `Bearer ${token}`;
  const res = await fetch(`${API_BASE}${path}`, { ...opts, headers });
  return res;
}

// ─── Auth state ─────────────────────────────────────────────────────────────
function loadAuth() {
  try {
    const raw = localStorage.getItem(TOKEN_KEY);
    if (!raw) return;
    const { token: t, user: u } = JSON.parse(raw);
    token = t; user = u;
  } catch { token = null; user = null; }
}

function persistAuth() {
  localStorage.setItem(TOKEN_KEY, JSON.stringify({ token, user }));
}

function clearAuth() {
  localStorage.removeItem(TOKEN_KEY);
  token = null; user = null;
}

function logout() {
  clearAuth();
  profile = null;
  applyAuthUI();
  questions = []; currentIdx = 0;
  document.getElementById('simulator-stage').style.display = 'none';
  document.getElementById('profile-form').style.display = 'block';
  scrollToHash('auth');
}

function scrollToHash(id) {
  const el = document.getElementById(id);
  if (el) el.scrollIntoView({ behavior: 'smooth', block: 'start' });
}

function applyAuthUI() {
  const isAuthed = !!token && !!user;
  document.getElementById('nav-cta-guest').style.display = isAuthed ? 'none' : 'inline-flex';
  document.getElementById('nav-user').style.display     = isAuthed ? 'flex' : 'none';
  document.getElementById('nav-dashboard').style.display = isAuthed ? 'inline' : 'none';
  document.getElementById('auth').style.display          = isAuthed ? 'none' : 'block';
  document.getElementById('dashboard').style.display     = isAuthed ? 'block' : 'none';
  if (isAuthed) document.getElementById('nav-user-name').textContent = user.name || user.email;
}

// ─── Auth handlers ──────────────────────────────────────────────────────────
function switchAuthTab(tab) {
  document.querySelectorAll('.auth-tab').forEach(el => {
    el.classList.toggle('active', el.dataset.tab === tab);
  });
  document.getElementById('form-signup').style.display = tab === 'signup' ? 'block' : 'none';
  document.getElementById('form-login').style.display  = tab === 'login'  ? 'block' : 'none';
  document.getElementById('auth-title').textContent = tab === 'signup' ? 'Crie sua conta' : 'Entre na sua conta';
}

async function doSignup() {
  const body = {
    name:            document.getElementById('su-name').value.trim(),
    email:           document.getElementById('su-email').value.trim().toLowerCase(),
    password:        document.getElementById('su-password').value,
    stack:           document.getElementById('su-stack').value,
    level:           document.getElementById('su-level').value,
    yearsExperience: parseInt(document.getElementById('su-years').value, 10) || 0,
    primaryLanguage: document.getElementById('su-lang').value,
    targetRole:      document.getElementById('su-target').value.trim(),
  };
  const errBox = document.getElementById('signup-error');
  errBox.textContent = '';
  const btn = document.getElementById('signup-submit');
  btn.disabled = true; btn.textContent = 'Criando...';
  try {
    const res = await apiFetch('/api/auth/signup', { method: 'POST', body: JSON.stringify(body) });
    if (!res.ok) {
      const text = await res.text();
      errBox.textContent = `Erro: ${text}`;
      return;
    }
    const data = await res.json();
    token = data.token; user = data.user;
    persistAuth();
    profileFromUser();
    applyAuthUI();
    await afterLoginFlow();
  } catch (e) {
    errBox.textContent = `Erro de rede: ${e.message}`;
  } finally {
    btn.disabled = false; btn.textContent = 'Criar conta →';
  }
}

async function doLogin() {
  const body = {
    email:    document.getElementById('li-email').value.trim().toLowerCase(),
    password: document.getElementById('li-password').value,
  };
  const errBox = document.getElementById('login-error');
  errBox.textContent = '';
  const btn = document.getElementById('login-submit');
  btn.disabled = true; btn.textContent = 'Entrando...';
  try {
    const res = await apiFetch('/api/auth/login', { method: 'POST', body: JSON.stringify(body) });
    if (!res.ok) { errBox.textContent = 'Credenciais inválidas'; return; }
    const data = await res.json();
    token = data.token; user = data.user;
    persistAuth();
    profileFromUser();
    applyAuthUI();
    await afterLoginFlow();
  } catch (e) {
    errBox.textContent = `Erro de rede: ${e.message}`;
  } finally {
    btn.disabled = false; btn.textContent = 'Entrar →';
  }
}

function profileFromUser() {
  if (!user) return;
  profile = {
    name:            user.name,
    stack:           user.stack,
    level:           user.level,
    yearsExperience: user.yearsExperience,
    primaryLanguage: user.primaryLanguage,
    targetRole:      user.targetRole,
  };
}

async function afterLoginFlow() {
  renderProfileCard();
  document.getElementById('profile-form').style.display = 'none';
  document.getElementById('simulator-stage').style.display = 'block';
  await Promise.all([loadQuestions(), loadHistory()]);
  scrollToHash('simulador');
}

// ─── Profile (form anônimo legacy + edição) ─────────────────────────────────
function showProfileForm(prefill) {
  document.getElementById('profile-form').style.display = 'block';
  document.getElementById('simulator-stage').style.display = 'none';
  document.getElementById('sim-status-label').innerHTML = '<span class="badge-dot"></span> setup';
  if (prefill) {
    document.getElementById('pf-name').value = prefill.name || '';
    document.getElementById('pf-years').value = prefill.yearsExperience ?? 2;
    document.getElementById('pf-stack').value = prefill.stack || 'backend';
    document.getElementById('pf-level').value = prefill.level || 'mid';
    document.getElementById('pf-lang').value = prefill.primaryLanguage || 'Go';
    document.getElementById('pf-target').value = prefill.targetRole || '';
  }
}

function showSimulatorStage() {
  document.getElementById('profile-form').style.display = 'none';
  document.getElementById('simulator-stage').style.display = 'block';
  document.getElementById('sim-status-label').innerHTML = '<span class="badge-dot"></span> profile mode';
}

function readProfileFromForm() {
  return {
    name: document.getElementById('pf-name').value.trim(),
    yearsExperience: parseInt(document.getElementById('pf-years').value, 10) || 0,
    stack: document.getElementById('pf-stack').value,
    level: document.getElementById('pf-level').value,
    primaryLanguage: document.getElementById('pf-lang').value,
    targetRole: document.getElementById('pf-target').value.trim(),
  };
}

async function saveProfile() {
  const p = readProfileFromForm();
  if (!p.name) return;
  profile = p;
  // Se logado: persiste no backend; senão guarda local
  if (token) {
    try {
      const res = await apiFetch('/api/me/update', { method: 'POST', body: JSON.stringify(p) });
      if (res.ok) {
        user = await res.json();
        persistAuth();
      }
    } catch (e) { console.warn('profile update failed:', e); }
  } else {
    localStorage.setItem(PROFILE_KEY_LEGACY, JSON.stringify(p));
  }
  renderProfileCard();
  showSimulatorStage();
  await loadQuestions();
}

function editProfile() { showProfileForm(profile); }

function renderProfileCard() {
  if (!profile) return;
  document.getElementById('pc-name').textContent = `${profile.name}`;
  const langPart = profile.primaryLanguage ? ` · ${profile.primaryLanguage}` : '';
  const yearsPart = profile.yearsExperience ? ` · ${profile.yearsExperience}y exp` : '';
  document.getElementById('pc-meta').textContent = `${profile.stack} · ${profile.level}${langPart}${yearsPart}`;
  const target = profile.targetRole || '';
  document.getElementById('pc-target').textContent = target ? `🎯 ${target}` : '';
  document.getElementById('pc-target').style.display = target ? 'block' : 'none';
}

// ─── Questions ──────────────────────────────────────────────────────────────
async function loadQuestions() {
  if (!profile) return;
  setQuestionLoading('Carregando perguntas ajustadas ao seu perfil…');
  try {
    const res = await apiFetch('/api/questions', { method: 'POST', body: JSON.stringify({ profile, limit: 5 }) });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const data = await res.json();
    questions = (data.questions && data.questions.length) ? data.questions : FALLBACK_QUESTIONS;
  } catch (err) {
    console.warn('falling back to local questions:', err);
    questions = FALLBACK_QUESTIONS;
  }
  currentIdx = 0;
  showQuestion();
}

function setQuestionLoading(msg) {
  document.getElementById('question-text').textContent = msg;
  const label = document.getElementById('question-box').querySelector('.sim-qlabel');
  if (label) label.textContent = '// loading…';
}

function showQuestion() {
  if (!questions.length) { setQuestionLoading('Nenhuma pergunta disponível.'); return; }
  const q = questions[currentIdx % questions.length];
  document.getElementById('question-text').innerHTML = q.text;
  const label = document.getElementById('question-box').querySelector('.sim-qlabel');
  if (label) label.textContent = `// question ${(currentIdx % questions.length) + 1} of ${questions.length} · ${q.topic}`;
  document.getElementById('answer-input').value = '';
  document.getElementById('char-count').textContent = '0 / 50 chars mínimos';
  hideFeedback();
}

function hideFeedback() { document.getElementById('feedback-panel').classList.remove('visible'); }
function stripHtml(html) { const t = document.createElement('div'); t.innerHTML = html; return t.textContent || t.innerText || ''; }
function currentQuestion() { return questions[currentIdx % questions.length]; }

// ─── Submit + feedback ──────────────────────────────────────────────────────
async function submitAnswer() {
  const input = document.getElementById('answer-input');
  const ans = input.value.trim();
  if (ans.length < 50) {
    input.style.borderColor = 'var(--err)';
    setTimeout(() => (input.style.borderColor = ''), 1500);
    return;
  }
  const btn = document.getElementById('submit-btn');
  const original = btn.textContent;
  btn.textContent = 'Avaliando...'; btn.disabled = true;
  try {
    const res = await apiFetch('/api/evaluate', {
      method: 'POST',
      body: JSON.stringify({
        level: profile?.level || 'junior',
        question: stripHtml(currentQuestion().text),
        answer: ans,
        profile,
      }),
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}: ${await res.text()}`);
    const fb = await res.json();
    renderFeedback(fb);
    if (token) loadHistory(); // refresh dashboard
  } catch (err) {
    renderError(err);
  } finally {
    btn.textContent = original; btn.disabled = false;
  }
}

function renderFeedback(fb) {
  document.getElementById('score-badge').textContent = fb.score;
  document.getElementById('score-badge').className = `score-badge ${fb.scoreClass}`;
  document.getElementById('fb-technical').textContent = fb.technical;
  document.getElementById('fb-english').innerHTML = (fb.english || []).map(e => `<li>${e}</li>`).join('');
  document.getElementById('fb-vocab').innerHTML = (fb.vocab || []).map(v => `<li>${v}</li>`).join('');
  const panel = document.getElementById('feedback-panel');
  panel.classList.add('visible');
  panel.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
}

function renderError(err) {
  document.getElementById('score-badge').textContent = 'erro';
  document.getElementById('score-badge').className = 'score-badge score-ok';
  document.getElementById('fb-technical').textContent =
    `Não foi possível contatar o backend (${err.message}).`;
  document.getElementById('fb-english').innerHTML = '';
  document.getElementById('fb-vocab').innerHTML = '';
  document.getElementById('feedback-panel').classList.add('visible');
}

function nextQuestion() { currentIdx++; showQuestion(); }

// ─── Speech-to-text (Web Speech API) ────────────────────────────────────────
let recognition = null;
let listening = false;
let baseText = '';

function toggleVoice() {
  const SR = window.SpeechRecognition || window.webkitSpeechRecognition;
  if (!SR) {
    alert('Reconhecimento de voz não suportado neste navegador. Use Chrome ou Edge.');
    return;
  }
  if (!recognition) {
    recognition = new SR();
    recognition.lang = 'en-US';
    recognition.continuous = true;
    recognition.interimResults = true;
    const input = document.getElementById('answer-input');
    recognition.onresult = (e) => {
      let interim = '';
      let final = '';
      for (let i = e.resultIndex; i < e.results.length; i++) {
        const t = e.results[i][0].transcript;
        if (e.results[i].isFinal) final += t + ' ';
        else interim += t;
      }
      if (final) baseText = (baseText + final).trimStart();
      input.value = (baseText + ' ' + interim).trim();
      input.dispatchEvent(new Event('input'));
    };
    recognition.onerror = (e) => console.warn('speech error:', e.error);
    recognition.onend = () => {
      // Auto-stop visual
      listening = false;
      document.getElementById('mic-btn').classList.remove('listening');
    };
  }
  if (listening) {
    recognition.stop();
    listening = false;
    document.getElementById('mic-btn').classList.remove('listening');
  } else {
    baseText = document.getElementById('answer-input').value;
    if (baseText) baseText += ' ';
    recognition.start();
    listening = true;
    document.getElementById('mic-btn').classList.add('listening');
  }
}

// ─── Dashboard / history ────────────────────────────────────────────────────
async function loadHistory() {
  if (!token) return;
  try {
    const res = await apiFetch('/api/history?limit=50');
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const items = await res.json();
    renderDashboard(items);
  } catch (e) {
    console.warn('history load failed:', e);
  }
}

function parseScore(s) {
  // "8.2/10" → 8.2
  const m = /([\d.]+)/.exec(s || '');
  return m ? parseFloat(m[1]) : null;
}

function renderDashboard(items) {
  if (!Array.isArray(items)) items = [];
  document.getElementById('ds-count').textContent = items.length;

  const scores = items.map(i => parseScore(i.score)).filter(x => x !== null);
  if (scores.length) {
    const avg = scores.reduce((a, b) => a + b, 0) / scores.length;
    document.getElementById('ds-avg').textContent = avg.toFixed(1);
    document.getElementById('ds-best').textContent = Math.max(...scores).toFixed(1);
  } else {
    document.getElementById('ds-avg').textContent = '—';
    document.getElementById('ds-best').textContent = '—';
  }

  // stack mais frequente
  const counts = {};
  items.forEach(i => { if (i.stack) counts[i.stack] = (counts[i.stack] || 0) + 1; });
  const topStack = Object.entries(counts).sort((a, b) => b[1] - a[1])[0];
  document.getElementById('ds-stack').textContent = topStack ? topStack[0] : '—';

  // sparkline: mais antigo → mais recente (esquerda → direita)
  const spark = document.getElementById('dash-spark');
  spark.innerHTML = '';
  const ordered = [...items].reverse();
  ordered.forEach(i => {
    const s = parseScore(i.score);
    if (s === null) return;
    const bar = document.createElement('div');
    bar.className = 'spark-bar' + (s < 7 ? ' low' : '');
    bar.style.height = `${Math.max(8, (s / 10) * 80)}px`;
    bar.dataset.score = `${s.toFixed(1)} · ${i.stack || '?'}`;
    spark.appendChild(bar);
  });
  if (!spark.children.length) {
    spark.innerHTML = '<span style="color:var(--muted);font-size:0.85rem;font-family:var(--font-mono);">// faça sua primeira avaliação pra ver o gráfico</span>';
  }

  const list = document.getElementById('dash-list');
  if (!items.length) {
    list.innerHTML = '<p class="section-sub" style="margin-top:1.5rem;">Você ainda não tem avaliações. Responda uma pergunta no simulador acima.</p>';
    return;
  }
  list.innerHTML = items.slice(0, 15).map(i => {
    const date = new Date(i.createdAt).toLocaleString('pt-BR', { dateStyle: 'short', timeStyle: 'short' });
    return `
      <div class="dash-item">
        <div class="dash-item-head">
          <div class="dash-item-meta">${date} · ${i.stack || '?'} · ${i.level || '?'}</div>
          <div class="score-badge ${i.scoreClass}">${i.score}</div>
        </div>
        <div class="dash-item-q">${i.question}</div>
        <div class="dash-item-tech">${i.technical}</div>
      </div>`;
  }).join('');
}

// ─── Boot ───────────────────────────────────────────────────────────────────
async function boot() {
  // textarea char counter
  document.getElementById('answer-input').addEventListener('input', function () {
    const len = this.value.length;
    const el = document.getElementById('char-count');
    el.textContent = `${len} / 50 chars mínimos`;
    el.style.color = len >= 50 ? 'var(--accent)' : 'var(--muted)';
  });

  loadAuth();
  applyAuthUI();

  if (token && user) {
    // Tenta refresh do user (caso tenha mudado no DB)
    try {
      const res = await apiFetch('/api/me');
      if (res.ok) { user = await res.json(); persistAuth(); }
      else if (res.status === 401) { clearAuth(); applyAuthUI(); }
    } catch {}
    profileFromUser();
    if (profile && profile.stack) {
      renderProfileCard();
      document.getElementById('profile-form').style.display = 'none';
      document.getElementById('simulator-stage').style.display = 'block';
      await Promise.all([loadQuestions(), loadHistory()]);
    } else {
      showProfileForm();
    }
  } else {
    // Modo anônimo (compatível com versão pré-auth via localStorage)
    try {
      const raw = localStorage.getItem(PROFILE_KEY_LEGACY);
      if (raw) profile = JSON.parse(raw);
    } catch {}
    if (profile) {
      renderProfileCard();
      showSimulatorStage();
      await loadQuestions();
    } else {
      showProfileForm();
    }
  }
}

document.addEventListener('DOMContentLoaded', boot);
