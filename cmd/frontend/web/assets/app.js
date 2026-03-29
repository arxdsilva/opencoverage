const projectList = document.getElementById('projectList');
const selectedProjectName = document.getElementById('selectedProjectName');
const selectedProjectMeta = document.getElementById('selectedProjectMeta');
const packagesBody = document.getElementById('packagesBody');
const runsBody = document.getElementById('runsBody');
const currentCoverage = document.getElementById('currentCoverage');
const previousCoverage = document.getElementById('previousCoverage');
const deltaCoverage = document.getElementById('deltaCoverage');
const thresholdPercent = document.getElementById('thresholdPercent');
const thresholdStatus = document.getElementById('thresholdStatus');
const refreshProjects = document.getElementById('refreshProjects');
const openHeatmap = document.getElementById('openHeatmap');
const closeHeatmap = document.getElementById('closeHeatmap');
const heatmapOverlay = document.getElementById('heatmapOverlay');
const heatmapGrid = document.getElementById('heatmapGrid');

let projects = [];
let selectedProjectId = null;
let heatmapItems = [];

refreshProjects.addEventListener('click', () => loadProjects());
openHeatmap.addEventListener('click', () => {
  const isOpen = heatmapOverlay.classList.contains('open');
  toggleHeatmapOverlay(!isOpen);
});
closeHeatmap.addEventListener('click', () => toggleHeatmapOverlay(false));

function toggleHeatmapOverlay(open) {
  heatmapOverlay.classList.toggle('open', open);
  heatmapOverlay.setAttribute('aria-hidden', String(!open));
}

async function loadProjects() {
  try {
    const res = await fetch('/api/projects');
    if (!res.ok) throw new Error(`failed to load projects (${res.status})`);
    const data = await res.json();
    projects = data.items || [];
    renderProjectList();

    if (!selectedProjectId && projects.length > 0) {
      await selectProject(projects[0].id);
    } else if (selectedProjectId) {
      await selectProject(selectedProjectId);
    }

    await loadHeatmap();
  } catch (err) {
    selectedProjectName.textContent = 'Failed to load projects';
    selectedProjectMeta.textContent = err.message;
    heatmapGrid.innerHTML = `<p class="muted">${err.message}</p>`;
  }
}

function renderProjectList() {
  projectList.innerHTML = '';

  if (projects.length === 0) {
    const li = document.createElement('li');
    li.textContent = 'No projects found.';
    li.className = 'muted';
    projectList.appendChild(li);
    return;
  }

  for (const project of projects) {
    const li = document.createElement('li');
    const btn = document.createElement('button');
    btn.className = selectedProjectId === project.id ? 'active' : '';
    btn.innerHTML = `<strong>${project.name || project.projectKey}</strong><small>${project.id}</small>`;
    btn.addEventListener('click', () => selectProject(project.id));
    li.appendChild(btn);
    projectList.appendChild(li);
  }
}

async function selectProject(projectId) {
  selectedProjectId = projectId;
  renderProjectList();
  renderHeatmap();

  const project = projects.find((p) => p.id === projectId);
  selectedProjectName.textContent = project?.name || project?.projectKey || 'Project';
  selectedProjectMeta.textContent = `${project?.projectKey || ''} - default branch: ${project?.defaultBranch || 'main'} - threshold: ${pct(project?.globalThresholdPercent)}`;

  await Promise.all([loadLatestComparison(projectId), loadRecentRuns(projectId)]);
}

async function loadHeatmap() {
  heatmapItems = [];
  renderHeatmapLoading();

  const items = await Promise.all(
    projects.map(async (project) => {
      try {
        const res = await fetch(`/api/projects/${project.id}/coverage-runs/latest-comparison`);
        if (!res.ok) throw new Error(`failed to load latest comparison (${res.status})`);
        const data = await res.json();
        return { project, comparison: data.comparison || null };
      } catch (err) {
        return { project, comparison: null, error: err.message };
      }
    }),
  );

  heatmapItems = items;
  renderHeatmap();
}

function renderHeatmapLoading() {
  heatmapGrid.innerHTML = '<p class="muted">Building heatmap...</p>';
}

function renderHeatmap() {
  heatmapGrid.innerHTML = '';

  if (projects.length === 0) {
    heatmapGrid.innerHTML = '<p class="muted">No projects found.</p>';
    return;
  }

  if (heatmapItems.length === 0) {
    renderHeatmapLoading();
    return;
  }

  const sorted = [...heatmapItems].sort((a, b) => {
    const ac = Number(a.comparison?.currentTotalCoveragePercent);
    const bc = Number(b.comparison?.currentTotalCoveragePercent);
    return (Number.isFinite(bc) ? bc : -1) - (Number.isFinite(ac) ? ac : -1);
  });

  for (const item of sorted) {
    const tile = buildHeatmapTile(item);
    heatmapGrid.appendChild(tile);
  }
}

function buildHeatmapTile(item) {
  const btn = document.createElement('button');
  const project = item.project;
  const name = project.name || project.projectKey;
  const current = Number(item.comparison?.currentTotalCoveragePercent);
  const delta = Number(item.comparison?.deltaPercent);
  const threshold = item.comparison?.thresholdStatus;
  const thresholdValue = Number(item.comparison?.thresholdPercent);

  const trendClass = heatTrendClass(current, delta, threshold);
  const size = tileSizeForCoverage(current);

  btn.className = `heat-tile ${trendClass} ${selectedProjectId === project.id ? 'selected' : ''}`;
  btn.style.setProperty('--col-span', String(size.col));
  btn.style.setProperty('--row-span', String(size.row));

  const deltaText = Number.isFinite(delta) ? signedPct(delta) : '-';
  btn.innerHTML = `
    <span class="heat-name">${name}</span>
    <span class="heat-value">${Number.isFinite(current) ? pct(current) : '-'}</span>
    <span class="heat-threshold">Threshold ${Number.isFinite(thresholdValue) ? pct(thresholdValue) : '-'}</span>
    <span class="heat-delta">${deltaText}</span>
  `;
  btn.title = `${name} | threshold=${threshold || 'unknown'} | delta=${deltaText}`;
  btn.addEventListener('click', async () => selectProject(project.id));
  return btn;
}

function heatTrendClass(current, delta, threshold) {
  if (!Number.isFinite(current)) return 'neutral';
  if ((Number.isFinite(delta) && delta < 0) || threshold === 'failed') return 'down';
  if ((Number.isFinite(delta) && delta >= 0) || threshold === 'passed') return 'up';
  return 'neutral';
}

function tileSizeForCoverage(current) {
  if (!Number.isFinite(current)) return { col: 3, row: 2 };
  if (current >= 90) return { col: 6, row: 4 };
  if (current >= 80) return { col: 5, row: 4 };
  if (current >= 70) return { col: 4, row: 3 };
  if (current >= 60) return { col: 4, row: 2 };
  return { col: 3, row: 2 };
}

async function loadLatestComparison(projectId) {
  packagesBody.innerHTML = '';
  try {
    const res = await fetch(`/api/projects/${projectId}/coverage-runs/latest-comparison`);
    if (!res.ok) throw new Error(`failed to load latest comparison (${res.status})`);
    const data = await res.json();

    currentCoverage.textContent = pct(data.comparison.currentTotalCoveragePercent);
    previousCoverage.textContent = data.comparison.previousTotalCoveragePercent == null ? '-' : pct(data.comparison.previousTotalCoveragePercent);
    deltaCoverage.textContent = data.comparison.deltaPercent == null ? '-' : signedPct(data.comparison.deltaPercent);
    thresholdPercent.textContent = pct(data.comparison.thresholdPercent);

    thresholdStatus.textContent = data.comparison.thresholdStatus || '-';
    thresholdStatus.className = `value ${data.comparison.thresholdStatus === 'passed' ? 'passed' : 'failed'}`;

    for (const p of data.packages || []) {
      const tr = document.createElement('tr');
      tr.innerHTML = `
        <td class="code">${p.importPath}</td>
        <td>${pct(p.currentCoveragePercent)}</td>
        <td>${p.previousCoveragePercent == null ? '-' : pct(p.previousCoveragePercent)}</td>
        <td>${p.deltaPercent == null ? '-' : signedPct(p.deltaPercent)}</td>
        <td class="${directionClass(p.direction)}">${p.direction || '-'}</td>
      `;
      packagesBody.appendChild(tr);
    }
  } catch (err) {
    currentCoverage.textContent = '-';
    previousCoverage.textContent = '-';
    deltaCoverage.textContent = '-';
    thresholdPercent.textContent = '-';
    thresholdStatus.textContent = 'error';
    thresholdStatus.className = 'value failed';

    const tr = document.createElement('tr');
    tr.innerHTML = `<td colspan="5" class="muted">${err.message}</td>`;
    packagesBody.appendChild(tr);
  }
}

async function loadRecentRuns(projectId) {
  runsBody.innerHTML = '';
  try {
    const res = await fetch(`/api/projects/${projectId}/coverage-runs?page=1&pageSize=20`);
    if (!res.ok) throw new Error(`failed to load runs (${res.status})`);
    const data = await res.json();

    for (const run of data.items || []) {
      const tr = document.createElement('tr');
      tr.innerHTML = `
        <td class="code">${run.id}</td>
        <td>${run.branch}</td>
        <td class="code">${run.commitSha}</td>
        <td>${pct(run.totalCoveragePercent)}</td>
        <td>${new Date(run.runTimestamp).toLocaleString()}</td>
      `;
      runsBody.appendChild(tr);
    }

    if ((data.items || []).length === 0) {
      const tr = document.createElement('tr');
      tr.innerHTML = '<td colspan="5" class="muted">No runs found.</td>';
      runsBody.appendChild(tr);
    }
  } catch (err) {
    const tr = document.createElement('tr');
    tr.innerHTML = `<td colspan="5" class="muted">${err.message}</td>`;
    runsBody.appendChild(tr);
  }
}

function pct(v) {
  if (v == null || Number.isNaN(v)) return '-';
  return `${Number(v).toFixed(2)}%`;
}

function signedPct(v) {
  const n = Number(v);
  if (Number.isNaN(n)) return '-';
  return `${n > 0 ? '+' : ''}${n.toFixed(2)}%`;
}

function directionClass(direction) {
  if (direction === 'up') return 'up';
  if (direction === 'down') return 'down';
  return 'equal';
}

loadProjects();
