const QUESTIONS = {
  junior: [
    { q: 'Explain the difference between <em>concurrency</em> and <em>parallelism</em>. Give a real-world example of each.', topic: 'backend' },
    { q: 'What is a <em>REST API</em>? What makes it RESTful?', topic: 'backend' },
    { q: 'Can you explain what <em>version control</em> is and why it\'s important?', topic: 'tooling' },
    { q: 'What is the difference between <em>SQL</em> and <em>NoSQL</em> databases?', topic: 'databases' },
    { q: 'What does <em>CI/CD</em> mean and why is it useful?', topic: 'devops' },
  ],
  mid: [
    { q: 'Describe how you would design a <em>rate limiter</em> for a public API.', topic: 'system design' },
    { q: 'Explain the concept of <em>eventual consistency</em> and when you\'d accept it over strong consistency.', topic: 'distributed systems' },
    { q: 'What are the trade-offs between <em>microservices</em> and a <em>monolith</em>?', topic: 'architecture' },
    { q: 'How does <em>container orchestration</em> with Kubernetes solve deployment challenges?', topic: 'devops' },
    { q: 'Walk me through how you would debug a memory leak in a production Go service.', topic: 'backend' },
  ],
  senior: [
    { q: 'How would you design a <em>distributed caching layer</em> that handles 100k requests/second? What are the failure modes?', topic: 'system design' },
    { q: 'Explain the <em>CAP theorem</em> and how it affects your choice of database in a multi-region deployment.', topic: 'distributed systems' },
    { q: 'What strategies would you use to ensure <em>zero-downtime deployments</em> in a Kubernetes cluster?', topic: 'devops' },
    { q: 'How do you approach <em>observability</em> in a microservices architecture? What\'s the difference between metrics, traces, and logs?', topic: 'observability' },
    { q: 'Describe a time you had to make a <em>significant architectural decision</em>. What trade-offs did you evaluate?', topic: 'leadership' },
  ],
};

const FEEDBACK = {
  junior: [
    {
      score: '7.5/10', cls: 'score-ok',
      tech: 'Good start — you correctly identified that concurrency is about dealing with multiple tasks at once and parallelism is about executing them simultaneously. To improve, explicitly mention that concurrency is a design concept (handling), while parallelism is an execution concept (doing). Mention Go goroutines as a concrete concurrency example.',
      english: ['Consider "I would say that..." → more natural: "In simple terms,..."', '"...makes it faster" → more precise: "...increases throughput"', 'Avoid starting sentences with "So" in a formal interview context.'],
      vocab: ['"handle multiple tasks" → "manage concurrent workloads"', '"at the same time" → "simultaneously" or "in parallel"', '"works well" → "performs efficiently" or "scales horizontally"'],
    },
  ],
  mid: [
    {
      score: '8.2/10', cls: 'score-good',
      tech: 'Strong answer. You covered the core concepts well and mentioned token bucket or sliding window — excellent. To push to a 9+, discuss distributed rate limiting (Redis + Lua scripts), and the trade-off between accuracy and latency when using a centralized store.',
      english: ['Phrase "like, basically" → remove filler words for a more confident delivery', '"we need to store" → "we maintain" or "we persist" (more formal)', 'Good use of "trade-off" — keep it up.'],
      vocab: ['"save counts" → "persist request counters"', '"block the request" → "reject with HTTP 429 Too Many Requests"', '"fast storage" → "low-latency data store (e.g., Redis)"'],
    },
  ],
  senior: [
    {
      score: '9.0/10', cls: 'score-good',
      tech: 'Excellent. You covered sharding, consistent hashing, cache invalidation strategies and mentioned circuit breakers for failure modes. For a complete answer: also address cache stampede (thundering herd) mitigation via probabilistic early expiration or request coalescing.',
      english: ['Very fluent overall. Minor: "in terms of" appears 3 times — vary with "regarding", "as for", "when it comes to"', '"we talked about" → "as I mentioned" (more professional)'],
      vocab: ['"lots of requests" → "high-throughput workloads"', '"cache misses piling up" → "cache stampede / thundering herd problem"', 'Good use of "hot key" and "eviction policy" — these are exactly the terms interviewers look for.'],
    },
  ],
};

let currentLevel = 'junior';
let currentIdx = 0;

function setLevel(btn) {
  document.querySelectorAll('.level-btn').forEach(b => b.classList.remove('active'));
  btn.classList.add('active');
  currentLevel = btn.dataset.level;
  currentIdx = 0;
  showQuestion();
  hideFeedback();
}

function showQuestion() {
  const qs = QUESTIONS[currentLevel];
  const q = qs[currentIdx % qs.length];
  document.getElementById('question-text').innerHTML = q.q;
  document.getElementById('question-box').querySelector('.sim-qlabel').textContent =
    `// question ${(currentIdx % qs.length) + 1} of ${qs.length} · ${q.topic}`;
  document.getElementById('answer-input').value = '';
  document.getElementById('char-count').textContent = '0 / 50 chars mínimos';
  hideFeedback();
}

function hideFeedback() {
  document.getElementById('feedback-panel').classList.remove('visible');
}

function submitAnswer() {
  const ans = document.getElementById('answer-input').value.trim();
  if (ans.length < 50) {
    document.getElementById('answer-input').style.borderColor = 'var(--err)';
    setTimeout(() => document.getElementById('answer-input').style.borderColor = '', 1500);
    return;
  }
  const fbs = FEEDBACK[currentLevel];
  const fb = fbs[currentIdx % fbs.length] || fbs[0];
  document.getElementById('score-badge').textContent = fb.score;
  document.getElementById('score-badge').className = `score-badge ${fb.cls}`;
  document.getElementById('fb-technical').textContent = fb.tech;
  document.getElementById('fb-english').innerHTML = fb.english.map(e => `<li>${e}</li>`).join('');
  document.getElementById('fb-vocab').innerHTML = fb.vocab.map(v => `<li>${v}</li>`).join('');
  document.getElementById('feedback-panel').classList.add('visible');
  document.getElementById('feedback-panel').scrollIntoView({ behavior: 'smooth', block: 'nearest' });
}

function nextQuestion() {
  currentIdx++;
  showQuestion();
}

document.addEventListener('DOMContentLoaded', () => {
  document.getElementById('answer-input').addEventListener('input', function () {
    const len = this.value.length;
    const el = document.getElementById('char-count');
    el.textContent = `${len} / 50 chars mínimos`;
    el.style.color = len >= 50 ? 'var(--accent)' : 'var(--muted)';
  });
});
