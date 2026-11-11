const state = {
  summary: null,
  jobs: [],
  caseDetail: null,
  currentJob: null,
  eventSource: null,
  caseIDs: null,
  jobStatus: {},
};

const $ = (selector, root = document) => root.querySelector(selector);
const $$ = (selector, root = document) => Array.from(root.querySelectorAll(selector));

function esc(value) {
  return String(value ?? '').replace(/[&<>"']/g, (char) => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;',
  }[char]));
}

function fileURL(path) {
  return `/api/file?path=${encodeURIComponent(path)}`;
}

function fmtDate(value) {
  if (!value) return '—';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '—';
  return date.toLocaleString('es', { dateStyle: 'short', timeStyle: 'short' });
}

function pct(value) {
  return Math.max(0, Math.min(100, Number(value) || 0));
}

async function api(path, options = {}) {
  const response = await fetch(path, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  });
  const text = await response.text();
  let data = null;
  if (text) {
    try { data = JSON.parse(text); } catch { data = { error: text }; }
  }
  if (!response.ok) {
    throw new Error(data?.error || `HTTP ${response.status}`);
  }
  return data;
}

function toast(message, kind = '') {
  const node = $('#toast');
  node.textContent = message;
  node.className = `toast show ${kind}`;
  clearTimeout(toast.timer);
  toast.timer = setTimeout(() => { node.className = 'toast'; }, 4200);
}

function setPage(title, subtitle) {
  $('#page-title').textContent = title;
  $('#page-subtitle').textContent = subtitle;
}

function setActiveNav(name) {
  $$('[data-nav]').forEach((link) => {
    link.classList.toggle('active', link.dataset.nav === name);
  });
}

function badge(value, prefix = '') {
  const normalized = String(value || 'neutral').toLowerCase();
  return `<span class="badge badge-${esc(normalized)}">${esc(prefix + normalized)}</span>`;
}

function verdictBadge(verdict, score) {
  return `${badge(verdict)} <span class="mono">${esc(score)}/100</span>`;
}

function progressHTML(percent, label) {
  return `
    <div class="progress"><span style="width:${pct(percent)}%"></span></div>
    <div class="progress-meta"><span>${esc(label || '')}</span><span>${pct(percent)}%</span></div>`;
}

function emptyHTML(title, text) {
  return `<div class="empty"><strong>${esc(title)}</strong>${esc(text || '')}</div>`;
}

function loadingHTML(text = 'Cargando…') {
  return `<div class="empty"><strong>${esc(text)}</strong></div>`;
}

function paintHealth(summary) {
  state.summary = summary;
  const ffmpeg = $('#health-ffmpeg');
  const chrome = $('#health-chrome');
  const workspace = $('#health-workspace');
  if (ffmpeg) ffmpeg.textContent = summary.ffmpeg ? 'ON' : 'OFF';
  if (chrome) chrome.textContent = summary.chrome ? 'ON' : 'OFF';
  if (workspace) workspace.textContent = summary.workspace ? summary.workspace.split(/[\\/]/).pop() : '—';
  const dotFfmpeg = $('#dot-ffmpeg');
  const dotChrome = $('#dot-chrome');
  if (dotFfmpeg) dotFfmpeg.classList.toggle('on', !!summary.ffmpeg);
  if (dotChrome) dotChrome.classList.toggle('on', !!summary.chrome);

  const chips = $('#chips');
  if (chips) {
    chips.innerHTML = `
      <span class="chip">Casos <strong>${summary.cases}</strong></span>
      <span class="chip">Jobs activos <strong>${summary.active_jobs}</strong></span>
      <span class="chip">FFmpeg <strong>${summary.ffmpeg ? 'ON' : 'OFF'}</strong></span>
      <span class="chip">Chromium <strong>${summary.chrome ? 'ON' : 'OFF'}</strong></span>`;
  }
}

async function refreshSummary() {
  try {
    paintHealth(await api('/api/summary'));
  } catch (error) {
    console.error(error);
  }
}

function applyRoute() {
  const [hashPart, queryPart] = location.hash.split('?');
  const hash = hashPart.replace(/^#\/?/, '');
  const parts = hash.split('/').filter(Boolean);
  state.routeQuery = new URLSearchParams(queryPart || '');
  const view = parts[0] || 'dashboard';
  setActiveNav(view);
  if (view !== 'job' && state.eventSource) {
    state.eventSource.close();
    state.eventSource = null;
  }
  return parts;
}

async function route() {
  const parts = applyRoute();
  const view = parts[0] || 'dashboard';
  const app = $('#app');
  try {
    if (view === 'dashboard') return await dashboard();
    if (view === 'new') return renderNewCase();
    if (view === 'jobs') return await jobsView();
    if (view === 'case' && parts[1]) return await caseView(decodeURIComponent(parts[1]));
    if (view === 'job' && parts[1]) return await jobView(decodeURIComponent(parts[1]));
    location.hash = '#/dashboard';
  } catch (error) {
    if (app) app.innerHTML = `<div class="error-box">${esc(error.message)}</div>`;
    toast(error.message, 'error');
  }
}

async function dashboard() {
  setPage('Dashboard', 'Casos, revisión y renders');
  const [summary, cases, jobs] = await Promise.all([
    api('/api/summary'), api('/api/cases'), api('/api/jobs'),
  ]);
  paintHealth(summary);
  state.jobs = jobs;

  const previousCases = state.caseIDs;
  const previousJobs = state.jobStatus || {};
  if (previousCases) {
    const fresh = cases.filter((item) => !previousCases.has(item.id));
    if (fresh.length) {
      toast(`Nuevo caso: ${fresh[0].title}`, 'ok');
    }
  }
  for (const job of jobs) {
    const previous = previousJobs[job.id];
    if (!previous) continue;
    if (previous !== 'done' && job.status === 'done') {
      toast(`Render completo: ${job.title}`, 'ok');
    }
    if (previous !== 'failed' && job.status === 'failed') {
      toast(`Render falló: ${job.title}`, 'error');
    }
  }
  state.caseIDs = new Set(cases.map((item) => item.id));
  state.jobStatus = Object.fromEntries(jobs.map((job) => [job.id, job.status]));

  const completed = cases.reduce((total, item) => total + (item.runs || 0), 0);
  const stats = `
    <section class="stats">
      <div class="stat"><div class="stat-label">Casos</div><div class="stat-value">${summary.cases}</div><div class="stat-note">engineering_case.json</div></div>
      <div class="stat"><div class="stat-label">Jobs activos</div><div class="stat-value">${summary.active_jobs}</div><div class="stat-note">${jobs.length} en esta sesión</div></div>
      <div class="stat"><div class="stat-label">Renders completos</div><div class="stat-value">${completed}</div><div class="stat-note">persistidos en runs/</div></div>
      <div class="stat"><div class="stat-label">Capacidades</div><div class="stat-value">${summary.ffmpeg ? 'MP4' : 'GIF'}</div><div class="stat-note">${summary.chrome ? 'Chromium disponible' : 'solo raster'}</div></div>
    </section>`;

  const casesPanel = `
    <section class="panel">
      <div class="panel-head"><div class="panel-title">Casos recientes</div><a class="btn btn-sm" href="#/new">Nuevo caso</a></div>
      <div class="panel-body tight">${cases.length ? `
        <table><thead><tr><th>Caso</th><th>Veredicto</th><th>Revisión</th><th>Runs</th><th>Fecha</th><th></th></tr></thead><tbody>
        ${cases.slice(0, 8).map((item) => `
          <tr>
            <td><div class="cell-title">${esc(item.title)}</div><div class="cell-sub mono">${esc(item.id.slice(0, 22))}…</div></td>
            <td>${verdictBadge(item.verdict, item.score)}</td>
            <td>${badge(item.review)}</td>
            <td class="mono">${item.runs}</td>
            <td class="cell-sub">${esc(fmtDate(item.generated_at))}</td>
            <td><a class="btn btn-sm" href="#/case/${encodeURIComponent(item.id)}">Abrir</a></td>
          </tr>`).join('')}
        </tbody></table>` : emptyHTML('Sin casos', 'Crea el primero desde “Nuevo caso”.')}</div>
    </section>`;

  const jobsPanel = `
    <section class="panel">
      <div class="panel-head"><div class="panel-title">Jobs</div><a class="btn btn-sm" href="#/jobs">Ver todos</a></div>
      <div class="panel-body tight">${jobs.length ? jobs.slice(0, 6).map(jobRowHTML).join('') : emptyHTML('Sin jobs', 'Los renders aparecerán aquí.')}</div>
    </section>`;

  $('#app').innerHTML = stats + casesPanel + jobsPanel;
}

function jobRowHTML(job) {
  return `
    <div class="job-row">
      <div class="job-main">
        <div class="job-title"><a href="#/job/${encodeURIComponent(job.id)}">${esc(job.title || job.id)}</a></div>
        <div class="job-meta">${esc(job.phase || '')} · ${esc(job.message || '')} ${job.total_clips ? `· ${job.clip}/${job.total_clips} clips` : ''}</div>
      </div>
      <div class="job-progress">${progressHTML(job.percent, job.status)}</div>
      <div>${badge(job.status)}</div>
    </div>`;
}

function renderNewCase() {
  setPage('Nuevo caso', 'Galileo: fuente → caso verificado');
  $('#app').innerHTML = `
    <section class="panel">
      <div class="panel-head"><div class="panel-title">Fuente primaria</div></div>
      <div class="panel-body">
        <div class="tabs" style="padding:0;margin-bottom:18px">
          <button class="tab active" data-source-tab="url">URL</button>
          <button class="tab" data-source-tab="text">Texto</button>
        </div>
        <form id="case-form" class="form-grid" data-mode="url">
          <div class="span-3" id="url-fields">
            <label>URL de la fuente
              <input name="url" placeholder="https://example.com/postmortem" autocomplete="off">
            </label>
            <div class="hint">Post-mortem, RFC o reporte de incidente público. Galileo bloquea destinos internos (SSRF).</div>
          </div>
          <div class="span-3" id="text-fields" hidden>
            <label>Texto de la fuente
              <textarea name="text" placeholder="Pega aquí el post-mortem, RFC o reporte…"></textarea>
            </label>
            <div class="form-grid" style="margin-top:14px">
              <label>Título <input name="title" placeholder="Incident: connection pool exhaustion"></label>
              <label>Tipo
                <select name="type">
                  <option value="postmortem">postmortem</option>
                  <option value="rfc">rfc</option>
                  <option value="adr">adr</option>
                  <option value="other">other</option>
                </select>
              </label>
            </div>
          </div>
          <div class="span-3 actions">
            <button class="btn btn-primary" type="submit">Extraer con Galileo</button>
            <span class="hint">La extracción usa el extractor determinista del MVP.</span>
          </div>
          <div class="span-3" id="case-error"></div>
        </form>
      </div>
    </section>`;

  let mode = 'url';
  $$('[data-source-tab]').forEach((button) => {
    button.addEventListener('click', () => {
      mode = button.dataset.sourceTab;
      $$('[data-source-tab]').forEach((item) => item.classList.toggle('active', item === button));
      $('#case-form').dataset.mode = mode;
      $('#url-fields').hidden = mode !== 'url';
      $('#text-fields').hidden = mode !== 'text';
    });
  });

  $('#case-form').addEventListener('submit', async (event) => {
    event.preventDefault();
    const form = event.currentTarget;
    const data = new FormData(form);
    const payload = mode === 'url'
      ? { url: String(data.get('url') || '').trim() }
      : { text: String(data.get('text') || ''), title: String(data.get('title') || ''), type: String(data.get('type') || 'other') };
    const errorBox = $('#case-error');
    errorBox.innerHTML = '';
    const button = $('button[type="submit"]', form);
    button.disabled = true;
    button.textContent = 'Extrayendo…';
    try {
      const result = await api('/api/cases', { method: 'POST', body: JSON.stringify(payload) });
      toast('Caso extraído correctamente', 'ok');
      location.hash = `#/case/${encodeURIComponent(result.id)}`;
    } catch (error) {
      errorBox.innerHTML = `<div class="error-box">${esc(error.message)}</div>`;
      toast(error.message, 'error');
    } finally {
      button.disabled = false;
      button.textContent = 'Extraer con Galileo';
    }
  });
}

async function caseView(id) {
  setPage('Caso', 'Revisión, dossier y render');
  const detail = await api(`/api/cases/${encodeURIComponent(id)}`);
  state.caseDetail = detail;
  const data = detail.case;
  const review = detail.review || { decision: 'pending', note: '' };

  $('#app').innerHTML = `
    <section class="panel">
      <div class="panel-head">
        <div>
          <div class="panel-title">${esc(data.title)}</div>
          <div class="cell-sub mono">${esc(data.id)}</div>
        </div>
        <div class="actions">
          ${verdictBadge(data.hype_filter.verdict, data.hype_filter.score)}
          ${badge(review.decision)}
        </div>
      </div>
      <div class="panel-body">
        <div class="grid-3">
          <div class="card danger"><h3>Síntoma</h3><p>${esc(data.symptom.summary)}</p></div>
          <div class="card warning"><h3>Causa</h3><p>${esc(data.cause.summary)}</p></div>
          <div class="card success"><h3>Solución</h3><p>${esc(data.solution.summary)}</p></div>
        </div>
        <div class="actions" style="margin-top:16px">
          <button class="btn btn-primary" id="approve-case">Aprobar</button>
          <button class="btn btn-danger" id="reject-case">Rechazar</button>
          <button class="btn" id="focus-render">Configurar render</button>
          ${detail.runs.length ? `<span class="hint">${detail.runs.length} run(s) disponibles</span>` : ''}
        </div>
        <div class="form-grid" style="margin-top:14px">
          <label class="span-3">Nota de revisión
            <input id="review-note" value="${esc(review.note)}" placeholder="Opcional: justifica la decisión">
          </label>
        </div>
      </div>
    </section>

    <section class="panel">
      <div class="tabs">
        <button class="tab active" data-case-tab="triad">Tríada</button>
        <button class="tab" data-case-tab="dossier">Dossier</button>
        <button class="tab" data-case-tab="cut">Guion</button>
        <button class="tab" data-case-tab="runs">Runs</button>
        <button class="tab" data-case-tab="render">Render</button>
      </div>
      <div class="panel-body" id="case-tab-body"></div>
    </section>`;

  const body = $('#case-tab-body');
  const showTab = async (tab) => {
    $$('[data-case-tab]').forEach((button) => button.classList.toggle('active', button.dataset.caseTab === tab));
    if (tab === 'triad') {
      body.innerHTML = triadHTML(data);
      return;
    }
    if (tab === 'render') {
      body.innerHTML = renderFormHTML(data);
      wireRenderForm(id);
      return;
    }
    if (tab === 'dossier') {
      body.innerHTML = await dossierTabHTML(detail);
      return;
    }
    if (tab === 'cut') {
      body.innerHTML = await cutTabHTML(detail);
      return;
    }
    if (tab === 'runs') {
      body.innerHTML = runsTabHTML(detail);
      wireClipPreviews();
    }
  };

  $$('[data-case-tab]').forEach((button) => {
    button.addEventListener('click', () => showTab(button.dataset.caseTab));
  });

  $('#approve-case').addEventListener('click', () => saveReview(id, 'approved'));
  $('#reject-case').addEventListener('click', () => saveReview(id, 'rejected'));
  $('#focus-render').addEventListener('click', () => {
    showTab('render');
    $('[data-case-tab="render"]').scrollIntoView({ behavior: 'smooth', block: 'start' });
  });

  showTab('triad');
  const requestedTab = state.routeQuery?.get('tab');
  if (requestedTab) showTab(requestedTab);
}

async function saveReview(id, decision) {
  const note = $('#review-note')?.value || '';
  try {
    await api(`/api/cases/${encodeURIComponent(id)}/review`, {
      method: 'POST',
      body: JSON.stringify({ decision, note }),
    });
    toast(`Caso ${decision === 'approved' ? 'aprobado' : 'rechazado'}`, 'ok');
    await caseView(id);
  } catch (error) {
    toast(error.message, 'error');
  }
}

function evidenceHTML(evidence) {
  if (!evidence || !evidence.length) return '<div class="hint">Sin evidencia citada.</div>';
  return `<div class="evidence">${evidence.map((item) => `
    <blockquote>${esc(item.quote)}<div class="locator">${esc(item.locator || '')}</div></blockquote>`).join('')}</div>`;
}

function triadHTML(data) {
  const flags = data.hype_filter.flags || [];
  const unsupported = data.hype_filter.unsupported_claims || [];
  return `
    <div class="grid-2">
      <div class="card danger">
        <h3>Síntoma · ${data.symptom.observable ? 'observable' : 'no observable'}</h3>
        <p>${esc(data.symptom.summary)}</p>
        ${evidenceHTML(data.symptom.evidence)}
      </div>
      <div class="card warning">
        <h3>Causa</h3>
        <p>${esc(data.cause.summary)}</p>
        ${data.cause.mechanism ? `<div class="hint" style="margin-top:10px">Mecanismo: ${esc(data.cause.mechanism)}</div>` : ''}
        ${evidenceHTML(data.cause.evidence)}
      </div>
    </div>
    <div class="grid-2" style="margin-top:16px">
      <div class="card success">
        <h3>Solución</h3>
        <p>${esc(data.solution.summary)}</p>
        <div class="hint" style="margin-top:10px">Acciones</div>
        <ul>${(data.solution.actions || []).map((action) => `<li>${esc(action.description)}</li>`).join('')}</ul>
        <div class="hint" style="margin-top:10px">Trade-offs</div>
        <ul>${(data.solution.tradeoffs || []).map((item) => `<li>${esc(item)}</li>`).join('') || '<li>No declarados</li>'}</ul>
        <div class="hint" style="margin-top:10px">Verificación</div>
        <p>${esc(data.solution.verification || 'No declarada')}</p>
        ${evidenceHTML(data.solution.evidence)}
      </div>
      <div class="card neutral">
        <h3>Filtro Anti-Hype · ${esc(data.hype_filter.score)}/100</h3>
        <p>${esc(data.hype_filter.verdict)}</p>
        ${flags.length ? `<div class="flags">${flags.map((flag) => `
          <div class="flag"><span class="penalty">-${esc(flag.penalty)}</span><span><strong>${esc(flag.rule)}</strong><br>${esc(flag.message)}</span></div>`).join('')}</div>` : '<div class="hint" style="margin-top:10px">Sin flags.</div>'}
        ${unsupported.length ? `<div class="hint" style="margin-top:12px">Claims no soportados</div><ul>${unsupported.map((item) => `<li>${esc(item)}</li>`).join('')}</ul>` : ''}
      </div>
    </div>`;
}

function latestRun(detail) {
  return detail.runs && detail.runs.length ? detail.runs[0] : null;
}

async function outputJSON(run, kind) {
  const output = (run.outputs || []).find((item) => item.kind === kind);
  if (!output) return null;
  return api(fileURL(output.path));
}

async function dossierTabHTML(detail) {
  const run = latestRun(detail);
  if (!run) return emptyHTML('Sin dossier', 'Ejecuta un render para generar dossier_tecnico.json.');
  const dossier = await outputJSON(run, 'dossier');
  if (!dossier) return emptyHTML('Sin dossier', 'El run no contiene dossier_tecnico.json.');
  return `
    <p>${esc(dossier.summary)}</p>
    <table style="margin-top:14px"><thead><tr><th>Dato</th><th>Valor</th><th>Fuente</th></tr></thead><tbody>
      ${dossier.facts.map((fact) => `<tr><td><strong>${esc(fact.label)}</strong></td><td>${esc(fact.value)}</td><td class="mono">${esc(fact.source)}</td></tr>`).join('')}
    </tbody></table>
    <div class="grid-2" style="margin-top:16px">
      ${dossier.artifacts.map((artifact) => `
        <div class="card">
          <h3>${esc(artifact.kind)} · ${esc(artifact.title)}</h3>
          <pre>${esc(artifact.content)}</pre>
        </div>`).join('')}
    </div>`;
}

async function cutTabHTML(detail) {
  const run = latestRun(detail);
  if (!run) return emptyHTML('Sin guion', 'Ejecuta un render para generar director_cut.json.');
  const cut = await outputJSON(run, 'director_cut');
  if (!cut) return emptyHTML('Sin guion', 'El run no contiene director_cut.json.');
  return `${cut.scenes.map((scene) => `
    <div class="panel" style="box-shadow:none;margin-bottom:14px">
      <div class="panel-head"><div class="panel-title">${esc(scene.id)} · ${esc(scene.title)}</div><span class="hint">${scene.clips.length} clips</span></div>
      <div class="panel-body">
        ${scene.clips.map((clip) => `
          <div class="card" style="margin-bottom:10px">
            <h3>${esc(clip.id)} · ${esc(clip.visual.kind)}</h3>
            <p><strong>${esc(clip.visual.title)}</strong> · ${esc(clip.duration_sec)} s</p>
            <div class="hint" style="margin-top:6px">${esc(clip.narration)}</div>
          </div>`).join('')}
      </div>
    </div>`).join('')}`;
}

function runsTabHTML(detail) {
  if (!detail.runs.length) return emptyHTML('Sin runs', 'Lanza el primer render desde la pestaña Render.');
  return detail.runs.map((run, index) => {
    const video = (run.outputs || []).find((item) => item.kind === 'video');
    const audio = (run.outputs || []).find((item) => item.kind === 'audio');
    const jsonOutputs = (run.outputs || []).filter((item) => item.kind !== 'video' && item.kind !== 'audio');
    return `
      <div class="panel" style="box-shadow:none;margin-bottom:16px">
        <div class="panel-head">
          <div><div class="panel-title">${esc(run.id)}</div><div class="cell-sub">${esc(fmtDate(run.finished_at))} · ${esc(run.frames)} frames</div></div>
          ${index === 0 ? badge('latest') : ''}
        </div>
        <div class="panel-body">
          ${video ? (video.path.endsWith('.gif')
            ? `<img src="${fileURL(video.path)}" alt="render" style="max-width:100%;border:1px solid var(--line);border-radius:8px">`
            : `<video controls src="${fileURL(video.path)}"></video>`) : '<div class="hint">Sin vídeo.</div>'}
          ${audio ? `<audio controls src="${fileURL(audio.path)}"></audio>` : ''}
          <div class="actions" style="margin-top:12px">
            ${jsonOutputs.map((item) => `<a class="btn btn-sm" href="${fileURL(item.path)}" target="_blank" rel="noopener">${esc(item.kind)}</a>`).join('')}
            ${video ? `<a class="btn btn-sm" href="${fileURL(video.path)}" download>Descargar vídeo</a>` : ''}
          </div>
          ${run.clips && run.clips.length ? `
            <div class="form-grid" style="margin-top:16px">
              <label class="span-2">Previsualizar HTML de un clip
                <select id="clip-select-${esc(run.id)}" data-clip-select>
                  <option value="">Selecciona un clip…</option>
                  ${run.clips.filter((clip) => clip.html).map((clip) => `<option value="${fileURL(clip.html)}">${esc(clip.id)}</option>`).join('')}
                </select>
              </label>
            </div>
            <iframe class="clip-preview" data-clip-frame="${esc(run.id)}" title="preview"></iframe>` : ''}
        </div>
      </div>`;
  }).join('');
}

function wireClipPreviews() {
  $$('[data-clip-select]').forEach((select) => {
    select.addEventListener('change', () => {
      const frame = $(`[data-clip-frame="${CSS.escape(select.id.replace('clip-select-', ''))}"]`);
      if (frame) frame.src = select.value;
    });
  });
}

function renderFormHTML(data) {
  const summary = state.summary || {};
  const ttsURL = summary.tts_url || 'http://localhost:8000';
  return `
    <div class="note-box">
      Para un vídeo de 12 min a 1080p60 usa <strong>20 s por clip × 4 copias</strong>.
      ${summary.ffmpeg ? 'FFmpeg está disponible: el cierre será MP4.' : 'Sin FFmpeg: el cierre será GIF + WAV.'}
    </div>
    <form id="render-form" class="form-grid" style="margin-top:18px">
      <label>Segundos por clip <input name="clip_seconds" type="number" min="0.25" max="30" step="0.25" value="2.5"></label>
      <label>Copias de la secuencia <input name="clip_copies" type="number" min="1" max="100" value="1"></label>
      <label>FPS <input name="fps" type="number" min="1" max="120" value="12"></label>
      <label>Ancho <input name="width" type="number" min="64" max="3840" value="640"></label>
      <label>Alto <input name="height" type="number" min="64" max="2160" value="360"></label>
      <label>Renderer
        <select name="renderer">
          <option value="raster">raster (Go puro)</option>
          <option value="chromium">chromium</option>
        </select>
      </label>
      <label>Encoder
        <select name="encoder">
          <option value="auto">auto</option>
          <option value="ffmpeg">ffmpeg / MP4</option>
          <option value="gif">gif</option>
        </select>
      </label>
      <label>TTS
        <select name="tts">
          <option value="tone">tone (offline)</option>
          <option value="http">http</option>
        </select>
      </label>
      <label>Voz <input name="voice" value="default"></label>
      <label>Atlas
        <select name="atlas">
          <option value="deterministic">deterministic</option>
          <option value="github">github</option>
        </select>
      </label>
      <label>Repo GitHub <input name="repo" placeholder="owner/name"></label>
      <label>TTS URL <input name="tts_url" value="${esc(ttsURL)}" placeholder="http://localhost:8000"></label>
      <label class="span-2">Chrome path (Chromium) <input name="chrome" placeholder="autodetectado"></label>
      <label class="span-2">Ref GitHub <input name="ref" placeholder="rama/tag/commit"></label>
      <div class="span-3 actions">
        <button class="btn btn-primary" type="submit">Lanzar render</button>
        <span class="hint">Se ejecuta en background; puedes seguir el progreso en Jobs.</span>
      </div>
      <div class="span-3" id="render-error"></div>
    </form>`;
}

function wireRenderForm(id) {
  const form = $('#render-form');
  if (!form) return;
  form.addEventListener('submit', async (event) => {
    event.preventDefault();
    const body = new FormData(form);
    const payload = {
      clip_seconds: Number(body.get('clip_seconds')),
      clip_copies: Number(body.get('clip_copies')),
      fps: Number(body.get('fps')),
      width: Number(body.get('width')),
      height: Number(body.get('height')),
      renderer: body.get('renderer'),
      encoder: body.get('encoder'),
      tts: body.get('tts'),
      voice: body.get('voice'),
      atlas: body.get('atlas'),
      repo: body.get('repo'),
      ref: body.get('ref'),
      tts_url: body.get('tts_url'),
      chrome: body.get('chrome'),
    };
    const errorBox = $('#render-error');
    errorBox.innerHTML = '';
    const button = $('button[type="submit"]', form);
    button.disabled = true;
    button.textContent = 'Lanzando…';
    try {
      const job = await api(`/api/cases/${encodeURIComponent(id)}/render`, {
        method: 'POST', body: JSON.stringify(payload),
      });
      toast('Render en cola', 'ok');
      location.hash = `#/job/${encodeURIComponent(job.id)}`;
    } catch (error) {
      errorBox.innerHTML = `<div class="error-box">${esc(error.message)}</div>`;
      toast(error.message, 'error');
    } finally {
      button.disabled = false;
      button.textContent = 'Lanzar render';
    }
  });
}

async function jobsView() {
  setPage('Jobs', 'Cola y progreso de renders');
  const jobs = await api('/api/jobs');
  state.jobs = jobs;
  $('#app').innerHTML = `
    <section class="panel">
      <div class="panel-head"><div class="panel-title">Todos los jobs</div><button class="btn btn-sm" id="refresh-jobs">Actualizar</button></div>
      <div class="panel-body tight">${jobs.length ? jobs.map(jobRowHTML).join('') : emptyHTML('Sin jobs', 'Lanza un render desde un caso.')}</div>
    </section>`;
  $('#refresh-jobs').addEventListener('click', jobsView);
}

async function jobView(id) {
  setPage('Render', 'Progreso en vivo');
  state.currentJob = await api(`/api/jobs/${encodeURIComponent(id)}`);
  paintJob(state.currentJob);
  if (state.currentJob.status === 'done' || state.currentJob.status === 'failed') return;

  if (state.eventSource) state.eventSource.close();
  const source = new EventSource(`/api/jobs/${encodeURIComponent(id)}/events`);
  state.eventSource = source;
  source.onmessage = (event) => {
    try {
      const job = JSON.parse(event.data);
      state.currentJob = job;
      paintJob(job);
      if (job.status === 'done' || job.status === 'failed') {
        source.close();
        state.eventSource = null;
        refreshSummary();
      }
    } catch (error) {
      console.error(error);
    }
  };
  source.onerror = () => {
    source.close();
    state.eventSource = null;
    pollJob(id);
  };
}

async function pollJob(id) {
  try {
    const job = await api(`/api/jobs/${encodeURIComponent(id)}`);
    state.currentJob = job;
    paintJob(job);
    if (job.status === 'queued' || job.status === 'running') {
      setTimeout(() => pollJob(id), 2000);
    }
  } catch (error) {
    toast(error.message, 'error');
  }
}

function paintJob(job) {
  const outputs = job.outputs || [];
  const video = outputs.find((item) => item.kind === 'video');
  const audio = outputs.find((item) => item.kind === 'audio');
  const others = outputs.filter((item) => item.kind !== 'video' && item.kind !== 'audio');
  const clipText = job.total_clips ? `Clip ${job.clip}/${job.total_clips}` : '';
  $('#app').innerHTML = `
    <section class="panel">
      <div class="panel-head">
        <div><div class="panel-title">${esc(job.title || job.id)}</div><div class="cell-sub mono">${esc(job.id)}</div></div>
        <div class="actions">${badge(job.status)}<a class="btn btn-sm" href="#/case/${encodeURIComponent(job.public_case_id)}">Ver caso</a></div>
      </div>
      <div class="panel-body">
        ${progressHTML(job.percent, [job.phase, job.message, clipText].filter(Boolean).join(' · '))}
        ${job.error ? `<div class="error-box">${esc(job.error)}</div>` : ''}
        ${job.status === 'done' ? `
          <div class="grid-2" style="margin-top:18px">
            <div class="card success"><h3>Render completo</h3><p>${esc(fmtDate(job.finished_at))}</p><p class="hint">${job.percent}% · ${job.total_clips || 0} clips</p></div>
            <div class="card neutral"><h3>Salidas</h3><div class="actions">${others.map((item) => `<a class="btn btn-sm" href="${fileURL(item.path)}" target="_blank" rel="noopener">${esc(item.kind)}</a>`).join('')}</div></div>
          </div>
          ${video ? (video.path.endsWith('.gif')
            ? `<img src="${fileURL(video.path)}" alt="render" style="max-width:100%;margin-top:16px;border:1px solid var(--line);border-radius:8px">`
            : `<video controls src="${fileURL(video.path)}" style="margin-top:16px"></video>`) : ''}
          ${audio ? `<audio controls src="${fileURL(audio.path)}"></audio>` : ''}
        ` : ''}
      </div>
    </section>`;
}

window.addEventListener('hashchange', route);
window.addEventListener('beforeunload', () => {
  if (state.eventSource) state.eventSource.close();
});
window.addEventListener('DOMContentLoaded', async () => {
  await refreshSummary();
  if (!location.hash) location.hash = '#/dashboard';
  else route();
});
setInterval(async () => {
  if (document.hidden) return;
  try {
    if (location.hash.startsWith('#/dashboard')) {
      await dashboard();
    } else if (location.hash.startsWith('#/jobs')) {
      await jobsView();
    } else {
      await refreshSummary();
    }
  } catch (error) {
    console.error(error);
  }
}, 8000);
