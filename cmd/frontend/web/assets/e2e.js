const refreshProjects = document.getElementById('refreshProjects');
const projectSelector = document.getElementById('projectSelector');
const projectGroupFilter = document.getElementById('projectGroupFilter');
const projectSearchInput = document.getElementById('projectSearchInput');
const e2eScreenProjectTitle = document.getElementById('e2eScreenProjectTitle');
const e2eScreenProjectMeta = document.getElementById('e2eScreenProjectMeta');
const e2eStatus = document.getElementById('e2eStatus');
const e2ePassRate = document.getElementById('e2ePassRate');
const e2eFailedSpecsCount = document.getElementById('e2eFailedSpecsCount');
const e2eDuration = document.getElementById('e2eDuration');
const e2eBranchFilter = document.getElementById('e2eBranchFilter');
const e2eStatusFilter = document.getElementById('e2eStatusFilter');
const e2eEnvironmentFilter = document.getElementById('e2eEnvironmentFilter');
const e2ePlatformFilter = document.getElementById('e2ePlatformFilter');
const e2eSpecTypeFilter = document.getElementById('e2eSpecTypeFilter');
const e2eReload = document.getElementById('e2eReload');
const e2eAutoRefreshInterval = document.getElementById('e2eAutoRefreshInterval');
const e2eAutoRefreshStatus = document.getElementById('e2eAutoRefreshStatus');
const e2eAutoRefreshProgressBar = document.getElementById('e2eAutoRefreshProgressBar');
const e2eRunChain = document.getElementById('e2eRunChain');
const e2eRunsBody = document.getElementById('e2eRunsBody');
const e2eFailedSpecsBody = document.getElementById('e2eFailedSpecsBody');
const openE2EHeatmap = document.getElementById('openE2EHeatmap');
const closeE2EHeatmap = document.getElementById('closeE2EHeatmap');
const e2eHeatmapOverlay = document.getElementById('e2eHeatmapOverlay');
const heatmapBranchFilter = document.getElementById('heatmapBranchFilter');
const heatmapStatusFilter = document.getElementById('heatmapStatusFilter');
const heatmapSpecTypeFilter = document.getElementById('heatmapSpecTypeFilter');
const heatmapReload = document.getElementById('heatmapReload');
const e2eHeatmap = document.getElementById('e2eHeatmap');
const appShell = document.getElementById('appShell');
const toggleSidebar = document.getElementById('toggleSidebar');

let projects = [];
let filteredProjects = [];
let selectedProjectId = null;
let selectedE2ERunId = null;
let currentE2ERunItems = [];
const e2eRunChainMaxItems = 5;
const allGroupsFilterValue = '__all__';
const ungroupedFilterValue = '__ungrouped__';
const sidebarCollapsedKey = 'opencoverage.sidebarCollapsed.e2e';
const e2eAutoRefreshStorageKey = 'opencoverage.autoRefresh.e2e';
const e2eDefaultAutoRefreshInterval = '60s';
const e2eAutoRefreshIntervals = Object.freeze({
  off: 0,
  '15s': 15000,
  '30s': 30000,
  '60s': 60000,
  '5m': 300000,
});
let e2eRefreshTimeoutId = 0;
let e2eRefreshInFlight = false;
let e2eRefreshCountdownIntervalId = 0;
let e2eNextRefreshAt = 0;
let e2eRefreshDurationMs = 0;

refreshProjects.addEventListener('click', async () => {
  await performE2ERefresh('manual');
});
e2eAutoRefreshInterval.addEventListener('change', () => {
  persistE2EAutoRefreshInterval(e2eAutoRefreshInterval.value);
  scheduleE2EAutoRefresh();
});
projectSelector.addEventListener('change', async (e) => {
  await selectProject(e.target.value);
});
projectGroupFilter.addEventListener('change', async () => {
  filterAndRenderProjects(projectSearchInput.value);
  await ensureSelectedProjectIsVisible();
});
projectSearchInput.addEventListener('input', (e) => {
  filterAndRenderProjects(e.target.value);
});
e2eBranchFilter.addEventListener('change', async () => {
  await loadE2EScreen(selectedProjectId, { preferredRunId: null });
});
e2eStatusFilter.addEventListener('change', async () => {
  await loadE2ERuns(selectedProjectId);
});
e2eEnvironmentFilter.addEventListener('change', async () => {
  await loadE2ERuns(selectedProjectId);
});
e2ePlatformFilter.addEventListener('change', async () => {
  await loadE2ERuns(selectedProjectId);
});
e2eSpecTypeFilter.addEventListener('change', async () => {
  await loadE2ERuns(selectedProjectId);
});
e2eReload.addEventListener('click', async () => {
  await runWithButtonBusy(e2eReload, 'Reload', 'Reloading...', async () => {
    await loadE2EScreen(selectedProjectId, { preferredRunId: selectedE2ERunId });
  });
});
openE2EHeatmap.addEventListener('click', async () => {
  const isOpen = e2eHeatmapOverlay.classList.contains('open');
  toggleE2EHeatmapOverlay(!isOpen);
  if (!isOpen) {
    await loadHeatmap();
  }
});
closeE2EHeatmap.addEventListener('click', () => toggleE2EHeatmapOverlay(false));
heatmapBranchFilter.addEventListener('change', async () => {
  await loadHeatmap();
});
heatmapStatusFilter.addEventListener('change', async () => {
  await loadHeatmap();
});
heatmapSpecTypeFilter.addEventListener('change', async () => {
  await loadHeatmap();
});
heatmapReload.addEventListener('click', async () => {
  await runWithButtonBusy(heatmapReload, 'Reload', 'Reloading...', async () => {
    await loadHeatmap();
  });
});
toggleSidebar.addEventListener('click', () => {
  const shouldCollapse = !appShell.classList.contains('sidebar-collapsed');
  setSidebarCollapsed(shouldCollapse);
});
document.addEventListener('visibilitychange', () => {
  if (document.hidden) {
    updateE2EAutoRefreshStatus();
    return;
  }

  if (e2eNextRefreshAt && Date.now() >= e2eNextRefreshAt && !e2eRefreshInFlight) {
    void performE2ERefresh('auto');
    return;
  }

  updateE2EAutoRefreshStatus();
});

initializeSidebarState();
initializeE2EAutoRefreshControl();

(async () => {
  await performE2ERefresh('initial');
  if (getQueryParam('heatmap') === 'open') {
    toggleE2EHeatmapOverlay(true);
    await loadHeatmap();
  }
})();

function getQueryParam(name) {
  const params = new URLSearchParams(window.location.search);
  return params.get(name);
}

function initializeE2EAutoRefreshControl() {
  const persisted = window.localStorage.getItem(e2eAutoRefreshStorageKey);
  const nextValue = Object.prototype.hasOwnProperty.call(e2eAutoRefreshIntervals, persisted)
    ? persisted
    : e2eDefaultAutoRefreshInterval;
  e2eAutoRefreshInterval.value = nextValue;
  updateE2EAutoRefreshStatus();
}

function getE2EAutoRefreshIntervalValue() {
  const selectedValue = e2eAutoRefreshInterval.value;
  return Object.prototype.hasOwnProperty.call(e2eAutoRefreshIntervals, selectedValue)
    ? selectedValue
    : e2eDefaultAutoRefreshInterval;
}

function getE2EAutoRefreshIntervalMs() {
  return e2eAutoRefreshIntervals[getE2EAutoRefreshIntervalValue()] || 0;
}

function persistE2EAutoRefreshInterval(value) {
  const nextValue = Object.prototype.hasOwnProperty.call(e2eAutoRefreshIntervals, value)
    ? value
    : e2eDefaultAutoRefreshInterval;
  window.localStorage.setItem(e2eAutoRefreshStorageKey, nextValue);
}

function clearE2EAutoRefresh() {
  if (!e2eRefreshTimeoutId) return;
  window.clearTimeout(e2eRefreshTimeoutId);
  e2eRefreshTimeoutId = 0;
}

function setE2EAutoRefreshProgress(progressRatio) {
  if (!e2eAutoRefreshProgressBar) return;
  const safeRatio = Math.max(0, Math.min(1, progressRatio));
  e2eAutoRefreshProgressBar.style.transform = `scaleX(${safeRatio})`;
}

function clearE2ECountdownTicker() {
  if (!e2eRefreshCountdownIntervalId) return;
  window.clearInterval(e2eRefreshCountdownIntervalId);
  e2eRefreshCountdownIntervalId = 0;
}

function formatRemainingTime(ms) {
  const totalSeconds = Math.max(0, Math.ceil(ms / 1000));
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  if (minutes > 0) {
    return `${minutes}m ${seconds}s`;
  }
  return `${seconds}s`;
}

function updateE2EAutoRefreshStatus() {
  if (!e2eAutoRefreshStatus) return;

  const intervalLabel = getE2EAutoRefreshIntervalValue();
  if (intervalLabel === 'off') {
    e2eAutoRefreshStatus.textContent = 'Auto refresh is off.';
    setE2EAutoRefreshProgress(0);
    return;
  }

  if (e2eRefreshInFlight) {
    e2eAutoRefreshStatus.textContent = `Refreshing now (${intervalLabel}).`;
    setE2EAutoRefreshProgress(0);
    return;
  }

  if (!e2eNextRefreshAt) {
    e2eAutoRefreshStatus.textContent = `Scheduled every ${intervalLabel}.`;
    setE2EAutoRefreshProgress(0);
    return;
  }

  const remainingMs = e2eNextRefreshAt - Date.now();
  e2eAutoRefreshStatus.textContent = `Next refresh in ${formatRemainingTime(remainingMs)} (${intervalLabel}).`;
  const denominator = e2eRefreshDurationMs || getE2EAutoRefreshIntervalMs() || 1;
  setE2EAutoRefreshProgress(remainingMs / denominator);
}

function scheduleE2EAutoRefresh() {
  clearE2EAutoRefresh();
  clearE2ECountdownTicker();
  e2eNextRefreshAt = 0;
  e2eRefreshDurationMs = 0;

  const intervalMs = getE2EAutoRefreshIntervalMs();
  if (!intervalMs) {
    updateE2EAutoRefreshStatus();
    return;
  }

  e2eRefreshDurationMs = intervalMs;
  e2eNextRefreshAt = Date.now() + intervalMs;
  setE2EAutoRefreshProgress(1);
  updateE2EAutoRefreshStatus();
  e2eRefreshCountdownIntervalId = window.setInterval(() => {
    updateE2EAutoRefreshStatus();
  }, 200);

  e2eRefreshTimeoutId = window.setTimeout(async () => {
    if (e2eRefreshInFlight) {
      scheduleE2EAutoRefresh();
      return;
    }

    await performE2ERefresh('auto');
  }, intervalMs);
}

function setE2ERefreshButtonBusy(busy) {
  refreshProjects.disabled = busy;
  refreshProjects.textContent = busy ? 'Refreshing...' : 'Refresh';
}

async function runWithButtonBusy(button, idleText, busyText, action) {
  if (e2eRefreshInFlight) {
    return false;
  }

  e2eRefreshInFlight = true;
  clearE2EAutoRefresh();
  clearE2ECountdownTicker();
  e2eNextRefreshAt = 0;
  e2eRefreshDurationMs = 0;
  updateE2EAutoRefreshStatus();
  button.disabled = true;
  button.textContent = busyText;
  try {
    await action();
    return true;
  } finally {
    button.disabled = false;
    button.textContent = idleText;
    e2eRefreshInFlight = false;
    scheduleE2EAutoRefresh();
  }
}

async function performE2ERefresh(source = 'manual') {
  if (e2eRefreshInFlight) {
    return false;
  }

  e2eRefreshInFlight = true;
  clearE2EAutoRefresh();
  clearE2ECountdownTicker();
  e2eNextRefreshAt = 0;
  e2eRefreshDurationMs = 0;
  updateE2EAutoRefreshStatus();

  const heatmapWasOpen = e2eHeatmapOverlay.classList.contains('open');

  if (source === 'manual') {
    setE2ERefreshButtonBusy(true);
  }

  try {
    await loadProjects();

    if (heatmapWasOpen) {
      await loadHeatmap();
    }

    return true;
  } finally {
    if (source === 'manual') {
      setE2ERefreshButtonBusy(false);
    }

    e2eRefreshInFlight = false;
    scheduleE2EAutoRefresh();
  }
}

function toggleE2EHeatmapOverlay(open) {
  e2eHeatmapOverlay.classList.toggle('open', open);
  e2eHeatmapOverlay.setAttribute('aria-hidden', String(!open));
}

function initializeSidebarState() {
  const persisted = window.localStorage.getItem(sidebarCollapsedKey);
  setSidebarCollapsed(persisted === 'true');
}

function setSidebarCollapsed(collapsed) {
  appShell.classList.toggle('sidebar-collapsed', collapsed);
  toggleSidebar.textContent = collapsed ? '▸' : '◂';
  toggleSidebar.setAttribute('aria-label', collapsed ? 'Expand sidebar' : 'Collapse sidebar');
  toggleSidebar.setAttribute('title', collapsed ? 'Expand sidebar' : 'Collapse sidebar');
  toggleSidebar.setAttribute('aria-expanded', String(!collapsed));
  window.localStorage.setItem(sidebarCollapsedKey, String(collapsed));
}

async function loadProjects() {
  try {
    const pageSize = 100;
    let page = 1;
    let totalPages = 1;
    const items = [];

    while (page <= totalPages) {
      const res = await fetch(`/api/projects?page=${page}&pageSize=${pageSize}`);
      if (!res.ok) throw new Error(`failed to load projects (${res.status})`);
      const data = await res.json();
      items.push(...(data.items || []));
      totalPages = Math.max(1, data.pagination?.totalPages || 1);
      page += 1;
    }

    projects = items;
    renderProjectGroupFilter();
    filterAndRenderProjects(projectSearchInput.value);

    const nextSelectedProjectId = filteredProjects.some((project) => project.id === selectedProjectId)
      ? selectedProjectId
      : (filteredProjects[0]?.id || null);

    if (!nextSelectedProjectId) {
      selectedProjectId = null;
      selectedE2ERunId = null;
      if (items.length === 0) {
        e2eScreenProjectTitle.textContent = 'No projects found';
        e2eScreenProjectMeta.textContent = 'Upload E2E runs to populate this view.';
        e2eRunChain.innerHTML = '<p class="muted">No E2E runs found.</p>';
        e2eRunsBody.innerHTML = '<tr><td colspan="10" class="muted">No E2E runs found.</td></tr>';
      } else {
        e2eScreenProjectTitle.textContent = 'No projects for current filter';
        e2eScreenProjectMeta.textContent = 'Adjust group and search filters to select a project.';
        e2eRunChain.innerHTML = '<p class="muted">No projects match current filters.</p>';
        e2eRunsBody.innerHTML = '<tr><td colspan="10" class="muted">No projects match current filters.</td></tr>';
      }
      e2eFailedSpecsBody.innerHTML = '<tr><td colspan="5" class="muted">No run selected.</td></tr>';
      e2eStatus.textContent = '-';
      e2eStatus.className = 'value';
      e2ePassRate.textContent = '-';
      e2eFailedSpecsCount.textContent = '-';
      e2eDuration.textContent = '-';
      renderProjectSelector();
    } else if (nextSelectedProjectId === selectedProjectId) {
      await selectProject(nextSelectedProjectId, { preferredRunId: selectedE2ERunId });
      renderProjectSelector();
    } else {
      await selectProject(nextSelectedProjectId);
      renderProjectSelector();
    }
  } catch (err) {
    e2eScreenProjectTitle.textContent = 'Failed to load projects';
    e2eScreenProjectMeta.textContent = err.message;
  }
}

function getProjectGroupValue(project) {
  const rawGroup = typeof project?.group === 'string' ? project.group.trim() : '';
  return rawGroup || ungroupedFilterValue;
}

function renderProjectGroupFilter() {
  const selectedValue = projectGroupFilter.value || allGroupsFilterValue;
  const groupValues = Array.from(new Set(projects.map((project) => getProjectGroupValue(project))));
  groupValues.sort((a, b) => {
    if (a === ungroupedFilterValue) return 1;
    if (b === ungroupedFilterValue) return -1;
    return a.localeCompare(b);
  });

  projectGroupFilter.innerHTML = '';

  const allOption = document.createElement('option');
  allOption.value = allGroupsFilterValue;
  allOption.textContent = 'All groups';
  projectGroupFilter.appendChild(allOption);

  for (const groupValue of groupValues) {
    const option = document.createElement('option');
    option.value = groupValue;
    option.textContent = groupValue === ungroupedFilterValue ? 'Ungrouped' : groupValue;
    projectGroupFilter.appendChild(option);
  }

  projectGroupFilter.value = [allGroupsFilterValue, ...groupValues].includes(selectedValue)
    ? selectedValue
    : allGroupsFilterValue;
}

function renderProjectSelector() {
  projectSelector.innerHTML = '';

  const emptyOption = document.createElement('option');
  emptyOption.value = '';
  emptyOption.textContent = 'Select a project...';
  projectSelector.appendChild(emptyOption);

  if (filteredProjects.length === 0) {
    const noResultsOption = document.createElement('option');
    noResultsOption.value = '';
    noResultsOption.textContent = 'No projects match current filters';
    noResultsOption.disabled = true;
    projectSelector.appendChild(noResultsOption);
  }

  for (const project of filteredProjects) {
    const option = document.createElement('option');
    option.value = project.id;
    option.textContent = `${project.name || project.projectKey} (${project.projectKey})`;
    projectSelector.appendChild(option);
  }

  projectSelector.value = selectedProjectId || '';
}

function filterAndRenderProjects(searchTerm) {
  const term = searchTerm.toLowerCase();
  const selectedGroup = projectGroupFilter.value || allGroupsFilterValue;
  filteredProjects = projects.filter((p) => {
    const groupMatches = selectedGroup === allGroupsFilterValue
      || getProjectGroupValue(p) === selectedGroup;
    if (!groupMatches) return false;
    if (!term) return true;

    const name = (p.name || '').toLowerCase();
    const key = (p.projectKey || '').toLowerCase();
    return name.includes(term) || key.includes(term);
  });

  renderProjectSelector();
}

async function ensureSelectedProjectIsVisible() {
  const selectedVisible = filteredProjects.some((project) => project.id === selectedProjectId);
  if (selectedVisible) {
    renderProjectSelector();
    return;
  }

  const nextProjectId = filteredProjects[0]?.id || null;
  if (!nextProjectId) {
    selectedProjectId = null;
    selectedE2ERunId = null;
    e2eScreenProjectTitle.textContent = 'No projects for current filter';
    e2eScreenProjectMeta.textContent = 'Adjust group and search filters to select a project.';
    e2eRunChain.innerHTML = '<p class="muted">No projects match current filters.</p>';
    e2eRunsBody.innerHTML = '<tr><td colspan="10" class="muted">No projects match current filters.</td></tr>';
    e2eFailedSpecsBody.innerHTML = '<tr><td colspan="5" class="muted">No run selected.</td></tr>';
    e2eStatus.textContent = '-';
    e2eStatus.className = 'value';
    e2ePassRate.textContent = '-';
    e2eFailedSpecsCount.textContent = '-';
    e2eDuration.textContent = '-';
    renderProjectSelector();
    return;
  }

  await selectProject(nextProjectId);
  renderProjectSelector();
}

function renderE2EBranchFilter(project, branches = []) {
  const selectedValue = e2eBranchFilter.value;
  e2eBranchFilter.innerHTML = '';

  const defaultBranch = project?.defaultBranch || 'main';
  const orderedBranches = Array.from(new Set([defaultBranch, ...branches.filter(Boolean)]));
  for (const branch of orderedBranches) {
    const option = document.createElement('option');
    option.value = branch;
    option.textContent = branch;
    e2eBranchFilter.appendChild(option);
  }

  e2eBranchFilter.value = orderedBranches.includes(selectedValue)
    ? selectedValue
    : (orderedBranches[0] || defaultBranch);
}

async function loadE2EBranches(projectId, defaultBranch) {
  try {
    const res = await fetch(`/api/projects/${projectId}/branches`);
    if (!res.ok) throw new Error(`failed to load branches (${res.status})`);
    const data = await res.json();
    const branches = Array.isArray(data.branches) ? data.branches.filter(Boolean) : [];
    return Array.from(new Set([defaultBranch, ...branches]));
  } catch (err) {
    return [defaultBranch];
  }
}

async function selectProject(projectId, options = {}) {
  const { preferredRunId = null } = options;

  if (!projectId) {
    selectedProjectId = null;
    selectedE2ERunId = null;
    e2eScreenProjectTitle.textContent = 'Select a project';
    e2eScreenProjectMeta.textContent = 'Choose a project from the left menu.';
    await loadE2EScreen(null);
    renderProjectSelector();
    return;
  }

  selectedProjectId = projectId;

  const project = projects.find((p) => p.id === projectId);
  e2eScreenProjectTitle.textContent = project?.name || project?.projectKey || 'Project';
  e2eScreenProjectMeta.textContent = `${project?.projectKey || ''} - default branch: ${project?.defaultBranch || 'main'}`;

  const defaultBranch = project?.defaultBranch || 'main';
  const branches = await loadE2EBranches(projectId, defaultBranch);
  renderE2EBranchFilter(project, branches);
  await loadE2EScreen(projectId, { preferredRunId });
}

async function loadE2EScreen(projectId, options = {}) {
  const { preferredRunId = selectedE2ERunId } = options;

  if (!projectId) {
    e2eRunChain.innerHTML = '<p class="muted">Select a project to view its run chain.</p>';
    e2eRunsBody.innerHTML = '<tr><td colspan="10" class="muted">Select a project first.</td></tr>';
    e2eFailedSpecsBody.innerHTML = '<tr><td colspan="5" class="muted">No run selected.</td></tr>';
    return;
  }

  await Promise.all([loadE2ELatestComparison(projectId), loadE2ERuns(projectId, preferredRunId)]);
}

async function loadE2ELatestComparison(projectId) {
  try {
    const requestedBranch = e2eBranchFilter.value || '';
    const url = new URL(`/api/projects/${projectId}/e2e-test-runs/latest-comparison`, window.location.origin);
    if (requestedBranch) {
      url.searchParams.set('branch', requestedBranch);
    }

    const res = await fetch(url.toString());
    if (!res.ok) throw new Error(`failed to load E2E comparison (${res.status})`);
    const data = await res.json();

    if (projectId !== selectedProjectId) return;
    if ((e2eBranchFilter.value || '') !== requestedBranch) return;

    e2eDuration.textContent = data.run?.durationMs == null ? '-' : `${Math.round(data.run.durationMs / 1000)}s`;
  } catch (err) {
    if (projectId !== selectedProjectId) return;
    e2eStatus.textContent = 'ERROR';
    e2eStatus.className = 'value failed';
    e2ePassRate.textContent = '-';
    e2eFailedSpecsCount.textContent = '-';
    e2eDuration.textContent = '-';
  }
}

async function loadE2ERuns(projectId, preferredRunId = null) {
  e2eRunChain.innerHTML = '';
  e2eRunsBody.innerHTML = '';
  currentE2ERunItems = [];

  const retainedRunId = preferredRunId || selectedE2ERunId;

  try {
    const url = new URL(`/api/projects/${projectId}/e2e-test-runs`, window.location.origin);
    url.searchParams.set('page', '1');
    url.searchParams.set('pageSize', '20');
    const project = projects.find((p) => p.id === projectId);
    const selectedBranch = e2eBranchFilter.value || project?.defaultBranch || 'main';
    const selectedStatus = e2eStatusFilter.value || '';
    const selectedEnvironment = e2eEnvironmentFilter.value || '';
    const selectedPlatform = e2ePlatformFilter.value || '';
    const selectedSpecType = e2eSpecTypeFilter.value || '';
    url.searchParams.set('branch', selectedBranch);
    if (selectedStatus) {
      url.searchParams.set('status', selectedStatus);
    }
    if (selectedEnvironment) {
      url.searchParams.set('environment', selectedEnvironment);
    }
    if (selectedPlatform) {
      url.searchParams.set('platform', selectedPlatform);
    }
    if (selectedSpecType) {
      url.searchParams.set('specType', selectedSpecType);
    }

    const res = await fetch(url.toString());
    if (!res.ok) throw new Error(`failed to load E2E runs (${res.status})`);
    const data = await res.json();
    const items = data.items || [];

    if (projectId !== selectedProjectId) return;
    const currentProject = projects.find((p) => p.id === projectId);
    const currentBranch = e2eBranchFilter.value || currentProject?.defaultBranch || 'main';
    const currentStatus = e2eStatusFilter.value || '';
    const currentEnvironment = e2eEnvironmentFilter.value || '';
    const currentPlatform = e2ePlatformFilter.value || '';
    const currentSpecType = e2eSpecTypeFilter.value || '';
    if (currentBranch !== selectedBranch || currentStatus !== selectedStatus || currentEnvironment !== selectedEnvironment || currentPlatform !== selectedPlatform || currentSpecType !== selectedSpecType) return;

    currentE2ERunItems = items;
    const passedRuns = items.filter((run) => run.status === 'passed').length;
    const failedRuns = items.filter((run) => run.status === 'failed').length;
    if (passedRuns === 0 && failedRuns === 0) {
      e2ePassRate.textContent = '-';
    } else if (failedRuns === 0) {
      e2ePassRate.textContent = '100%';
    } else {
      e2ePassRate.textContent = `${((passedRuns / (passedRuns + failedRuns)) * 100).toFixed(2)}%`;
    }

    if (items.length === 0) {
      selectedE2ERunId = null;
      e2eStatus.textContent = '-';
      e2eStatus.className = 'value';
      e2eFailedSpecsCount.textContent = '-';
      e2eRunChain.innerHTML = '<p class="muted">No E2E runs found for current filters.</p>';
      e2eRunsBody.innerHTML = '<tr><td colspan="10" class="muted">No E2E runs found.</td></tr>';
      e2eFailedSpecsBody.innerHTML = '<tr><td colspan="5" class="muted">No run selected.</td></tr>';
      return;
    }

    const latestRun = items[0];
    e2eStatus.textContent = (latestRun.status || '-').toUpperCase();
    e2eStatus.className = `value ${latestRun.status === 'passed' ? 'passed' : 'failed'}`;
    e2eFailedSpecsCount.textContent = String(latestRun.failedSpecs ?? '-');

    const nextSelectedRunId = retainedRunId && items.some((run) => run.id === retainedRunId)
      ? retainedRunId
      : items[0].id;

    selectedE2ERunId = nextSelectedRunId;
    renderE2ERunChain(items.slice(0, e2eRunChainMaxItems));

    for (const run of items) {
      const tr = document.createElement('tr');
      tr.dataset.runId = run.id;
      tr.innerHTML = `
        <td class="code">${run.id}</td>
        <td>${run.branch}</td>
        <td class="code">${run.commitSha}</td>
        <td class="${run.status === 'passed' ? 'up' : 'down'}">${run.status}</td>
        <td>${pct(run.passRatePercent)}</td>
        <td>${run.failedSpecs}</td>
        <td>${run.platformType || '-'}</td>
        <td>${run.testFramework || '-'}</td>
        <td>${run.environment || '-'}</td>
        <td>${new Date(run.runTimestamp).toLocaleString()}</td>
      `;
      tr.addEventListener('click', async () => {
        selectedE2ERunId = run.id;
        highlightSelectedRunRow();
        renderE2ERunChain(items.slice(0, e2eRunChainMaxItems));
        await loadE2ERunDetails(projectId, run.id);
      });
      e2eRunsBody.appendChild(tr);
    }

    highlightSelectedRunRow();
    await loadE2ERunDetails(projectId, selectedE2ERunId);
  } catch (err) {
    selectedE2ERunId = null;
    e2eRunChain.innerHTML = `<p class="muted">${err.message}</p>`;
    e2eRunsBody.innerHTML = `<tr><td colspan="10" class="muted">${err.message}</td></tr>`;
    e2eFailedSpecsBody.innerHTML = '<tr><td colspan="5" class="muted">Failed to load selected run details.</td></tr>';
    e2ePassRate.textContent = '-';
  }
}

function renderE2ERunChain(items) {
  if (!Array.isArray(items) || items.length === 0) {
    e2eRunChain.innerHTML = '<p class="muted">No E2E runs found for current filters.</p>';
    return;
  }

  const track = document.createElement('div');
  track.className = 'integration-run-chain-track';

  const displayItems = [...items].reverse();
  displayItems.forEach((run, index) => {
    const item = document.createElement('div');
    item.className = 'integration-chain-item';

    const button = document.createElement('button');
    button.type = 'button';
    button.className = `integration-chain-node ${run.status === 'passed' ? 'passed' : 'failed'}`;
    if (selectedE2ERunId === run.id) {
      button.classList.add('selected');
    }
    button.title = `${run.status.toUpperCase()} | ${formatDateTime(run.runTimestamp)} | ${pct(run.passRatePercent)}`;
    button.setAttribute('aria-label', `Run ${run.id}, ${run.status}, pass rate ${pct(run.passRatePercent)}`);
    button.addEventListener('click', async () => {
      selectedE2ERunId = run.id;
      highlightSelectedRunRow();
      renderE2ERunChain(items);
      await loadE2ERunDetails(selectedProjectId, run.id);
    });

    const label = document.createElement('p');
    label.className = 'integration-chain-label';
    label.textContent = `${shortCommit(run.commitSha)} · ${formatChainDate(run.runTimestamp)}`;

    item.appendChild(button);
    item.appendChild(label);
    track.appendChild(item);

    if (index < displayItems.length - 1) {
      const connector = document.createElement('span');
      connector.className = 'integration-chain-connector';
      connector.textContent = '→';
      connector.title = 'Oldest to newest';
      connector.setAttribute('aria-hidden', 'true');
      track.appendChild(connector);
    }
  });

  e2eRunChain.innerHTML = '';
  e2eRunChain.appendChild(track);
}

function formatChainDate(value) {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '-';

  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

function highlightSelectedRunRow() {
  const rows = e2eRunsBody.querySelectorAll('tr[data-run-id]');
  for (const row of rows) {
    row.classList.toggle('selected-row', row.dataset.runId === selectedE2ERunId);
  }
}

async function loadE2ERunDetails(projectId, runId) {
  e2eFailedSpecsBody.innerHTML = '';
  try {
    const res = await fetch(`/api/projects/${projectId}/e2e-test-runs/${runId}`);
    if (!res.ok) throw new Error(`failed to load E2E run details (${res.status})`);
    const data = await res.json();
    const failedSpecs = data.failedSpecs || [];
    if (failedSpecs.length === 0) {
      e2eFailedSpecsBody.innerHTML = '<tr><td colspan="5" class="muted">No failed specs for this run.</td></tr>';
      return;
    }

     for (const failed of failedSpecs) {
      const tr = document.createElement('tr');
      tr.innerHTML = `
        <td class="code">${escapeHtml(failed.specPath || '-')}</td>
        <td>${escapeHtml(failed.specType || '-')}</td>
        <td>${escapeHtml(failed.failureMessage || '-')}</td>
        <td class="code">${escapeHtml(failed.file || '-')}</td>
        <td>${failed.line || '-'}</td>
      `;
      e2eFailedSpecsBody.appendChild(tr);
    }
  } catch (err) {
    e2eFailedSpecsBody.innerHTML = `<tr><td colspan="5" class="muted">${err.message}</td></tr>`;
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

function shortCommit(commitSha) {
  if (!commitSha) return '-';
  return String(commitSha).slice(0, 7);
}

function formatDateTime(value) {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '-';
  return date.toLocaleString();
}

function escapeHtml(value) {
  return String(value)
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;');
}

function getProjectDefaultBranch(projectId) {
  const project = projects.find((p) => p.id === projectId);
  return project?.defaultBranch || 'main';
}

async function loadHeatmap() {
  e2eHeatmap.innerHTML = '<p class="muted">Loading heatmap…</p>';
  try {
    const url = new URL('/api/e2e-test-runs/heatmap', window.location.origin);
    url.searchParams.set('runsPerProject', '10');
    if (heatmapBranchFilter.value) url.searchParams.set('branch', heatmapBranchFilter.value);
    if (heatmapStatusFilter.value) url.searchParams.set('status', heatmapStatusFilter.value);
    if (heatmapSpecTypeFilter.value) url.searchParams.set('specType', heatmapSpecTypeFilter.value);

    const res = await fetch(url.toString());
    if (!res.ok) throw new Error(`heatmap request failed (${res.status})`);
    const data = await res.json();
    renderHeatmap(data.groups || []);
  } catch (err) {
    e2eHeatmap.innerHTML = `<p class="muted">${escapeHtml(err.message)}</p>`;
  }
}

function renderHeatmap(groups) {
  e2eHeatmap.innerHTML = '';

  if (groups.length === 0) {
    e2eHeatmap.innerHTML = '<p class="muted">No E2E runs found.</p>';
    return;
  }

  const environmentOrder = ['test', 'stage', 'prod'];

  for (const group of groups) {
    const groupEl = document.createElement('div');
    groupEl.className = 'integration-heatmap-group';

    const groupLabel = document.createElement('p');
    groupLabel.className = 'integration-heatmap-group-name';
    groupLabel.textContent = group.groupName || 'Ungrouped';
    groupEl.appendChild(groupLabel);

    for (const project of group.projects || []) {
      const defaultBranch = getProjectDefaultBranch(project.projectId);
      const runs = (project.runs || []).filter((run) => run.branch === defaultBranch);
      if (runs.length === 0) {
        continue;
      }

      const runsByEnvironment = {};
      runs.forEach((run) => {
        const env = run.environment || 'unspecified';
        if (!runsByEnvironment[env]) {
          runsByEnvironment[env] = [];
        }
        runsByEnvironment[env].push(run);
      });

      const sortedEnvironments = [
        ...environmentOrder.filter((env) => runsByEnvironment[env]),
        ...Object.keys(runsByEnvironment).filter((env) => !environmentOrder.includes(env)),
      ];

      const projectCardEl = document.createElement('section');
      projectCardEl.className = 'integration-heatmap-project-card';
      const newestProjectRun = runs[0] || null;
      if (newestProjectRun?.status === 'passed') {
        projectCardEl.classList.add('newest-passed');
      } else if (newestProjectRun?.status === 'failed') {
        projectCardEl.classList.add('newest-failed');
      }

      const projectHeaderEl = document.createElement('div');
      projectHeaderEl.className = 'integration-heatmap-project-header';

      const projectTitleEl = document.createElement('span');
      projectTitleEl.className = 'integration-heatmap-project-name';
      const projectDisplayName = project.projectName || project.projectKey;
      projectTitleEl.textContent = projectDisplayName;
      projectTitleEl.title = project.projectKey;
      projectHeaderEl.appendChild(projectTitleEl);

      const projectMetaEl = document.createElement('span');
      projectMetaEl.className = 'integration-heatmap-project-meta';
      projectMetaEl.textContent = `${sortedEnvironments.length} env${sortedEnvironments.length === 1 ? '' : 's'}`;
      projectHeaderEl.appendChild(projectMetaEl);

      projectCardEl.appendChild(projectHeaderEl);

      const environmentListEl = document.createElement('div');
      environmentListEl.className = 'integration-heatmap-environment-list';

      for (const environment of sortedEnvironments) {
        const envRuns = runsByEnvironment[environment];
        const rowEl = document.createElement('div');
        rowEl.className = 'integration-heatmap-environment-row';
        const newestRun = envRuns.length > 0 ? envRuns[0] : null;
        if (newestRun?.status === 'passed') {
          rowEl.classList.add('newest-passed');
        } else if (newestRun?.status === 'failed') {
          rowEl.classList.add('newest-failed');
        }

        const envInfoEl = document.createElement('div');
        envInfoEl.className = 'integration-heatmap-environment-info';

        const envBadgeEl = document.createElement('span');
        envBadgeEl.className = 'integration-heatmap-environment-badge';
        const envLabel = environment === 'unspecified' ? '(no env)' : environment;
        envBadgeEl.textContent = envLabel;
        envBadgeEl.title = `${project.projectKey} - Environment: ${environment}`;
        envInfoEl.appendChild(envBadgeEl);

        const envCountEl = document.createElement('span');
        envCountEl.className = 'integration-heatmap-environment-count';
        envCountEl.textContent = `${envRuns.length} run${envRuns.length === 1 ? '' : 's'}`;
        envInfoEl.appendChild(envCountEl);

        rowEl.appendChild(envInfoEl);

        const tilesEl = document.createElement('div');
        tilesEl.className = 'integration-heatmap-tiles';

        const displayRuns = [...envRuns].reverse();
        displayRuns.forEach((run, index) => {
          const tile = document.createElement('button');
          tile.type = 'button';
          tile.className = `integration-heatmap-tile ${run.status === 'passed' ? 'passed' : 'failed'}`;
          tile.textContent = run.status === 'passed' ? '✅' : '❌';
          if (selectedProjectId === project.projectId && selectedE2ERunId === run.id) {
            tile.classList.add('selected');
          }
          tile.title = [
            projectDisplayName,
            group.groupName ? `Group: ${group.groupName}` : null,
            `Environment: ${environment}`,
            `Branch: ${run.branch}`,
            `Commit: ${shortCommit(run.commitSha)}`,
            `${formatDateTime(run.runTimestamp)}`,
            `Status: ${run.status.toUpperCase()}`,
            `Pass rate: ${pct(run.passRatePercent)}`,
          ].filter(Boolean).join('\n');
          tile.setAttribute('aria-label', `${projectDisplayName} [${envLabel}] — ${run.status} — ${pct(run.passRatePercent)}`);

          tile.addEventListener('click', async () => {
            if (selectedProjectId !== project.projectId) {
              projectSelector.value = project.projectId;
              await selectProject(project.projectId, { preferredRunId: run.id });
              renderProjectSelector();
            } else {
              selectedE2ERunId = run.id;
              highlightSelectedRunRow();
              renderE2ERunChain(currentE2ERunItems);
              await loadE2ERunDetails(project.projectId, run.id);
            }
            renderHeatmap(groups);
          });

          tilesEl.appendChild(tile);

          if (index < displayRuns.length - 1) {
            const arrow = document.createElement('span');
            arrow.className = 'integration-heatmap-arrow';
            arrow.textContent = '→';
            arrow.title = 'Oldest to newest';
            arrow.setAttribute('aria-hidden', 'true');
            tilesEl.appendChild(arrow);
          }
        });

        rowEl.appendChild(tilesEl);
        environmentListEl.appendChild(rowEl);
      }

      projectCardEl.appendChild(environmentListEl);
      groupEl.appendChild(projectCardEl);
    }

    e2eHeatmap.appendChild(groupEl);
  }
}
