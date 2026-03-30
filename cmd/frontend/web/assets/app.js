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
const projectSelector = document.getElementById('projectSelector');
const projectSearchInput = document.getElementById('projectSearchInput');
const compareCard = document.getElementById('compareCard');
const compareSummary = document.getElementById('compareSummary');
const compareCurrent = document.getElementById('compareCurrent');
const compareBaseline = document.getElementById('compareBaseline');
const compareRunType = document.getElementById('compareRunType');
const branchSelector = document.getElementById('branchSelector');

let projects = [];
let allProjects = [];
let filteredProjects = [];
let selectedProjectId = null;
let selectedBranch = '';
let availableBranches = [];
let heatmapItems = [];
let trendChart = null;
let heatmapLayoutFrame = 0;

refreshProjects.addEventListener('click', () => loadProjects());
openHeatmap.addEventListener('click', () => {
  const isOpen = heatmapOverlay.classList.contains('open');
  toggleHeatmapOverlay(!isOpen);
});
closeHeatmap.addEventListener('click', () => toggleHeatmapOverlay(false));
projectSelector.addEventListener('change', async (e) => {
  await selectProject(e.target.value);
});
projectSearchInput.addEventListener('input', (e) => {
  filterAndRenderProjects(e.target.value);
});
branchSelector.addEventListener('change', async (e) => {
  selectedBranch = e.target.value;
  await loadLatestComparison(selectedProjectId);
});
window.addEventListener('resize', () => scheduleHeatmapLayout());

function toggleHeatmapOverlay(open) {
  heatmapOverlay.classList.toggle('open', open);
  heatmapOverlay.setAttribute('aria-hidden', String(!open));

  if (open) {
    scheduleHeatmapLayout();
  }
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
    allProjects = items;
    filteredProjects = items;
    renderProjectSelector();

    if (!selectedProjectId && projects.length > 0) {
      await selectProject(projects[0].id);
     renderProjectSelector(); // Update dropdown to show selected project
     } else if (selectedProjectId) {
       await selectProject(selectedProjectId);
       renderProjectSelector(); // Update dropdown to show selected project
     }

    await loadHeatmap();
  } catch (err) {
    selectedProjectName.textContent = 'Failed to load projects';
    selectedProjectMeta.textContent = err.message;
    heatmapGrid.innerHTML = `<p class="muted">${err.message}</p>`;
  }
}

function renderProjectSelector() {
  projectSelector.innerHTML = '';
  
  const emptyOption = document.createElement('option');
  emptyOption.value = '';
  emptyOption.textContent = 'Select a project...';
  projectSelector.appendChild(emptyOption);
  
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
  if (!term) {
    filteredProjects = allProjects;
  } else {
    filteredProjects = allProjects.filter(p => {
      const name = (p.name || '').toLowerCase();
      const key = (p.projectKey || '').toLowerCase();
      return name.includes(term) || key.includes(term);
    });
  }
  renderProjectSelector();
}

async function selectProject(projectId) {
  selectedProjectId = projectId;
  selectedBranch = '';
  projectSearchInput.value = '';
  renderHeatmap();

  const project = projects.find((p) => p.id === projectId) || allProjects.find((p) => p.id === projectId);
  selectedProjectName.textContent = project?.name || project?.projectKey || 'Project';
  selectedProjectMeta.textContent = `${project?.projectKey || ''} - default branch: ${project?.defaultBranch || 'main'} - threshold: ${pct(project?.globalThresholdPercent)}`;

  const defaultBranch = project?.defaultBranch || 'main';
  await Promise.all([loadBranches(projectId), loadRecentRuns(projectId), loadTrendChart(projectId, defaultBranch)]);
  await loadLatestComparison(projectId);
}

async function loadHeatmap() {
  heatmapItems = [];
  renderHeatmapLoading();

  const sourceProjects = allProjects.length > 0 ? allProjects : projects;

  const items = await Promise.all(
    sourceProjects.map(async (project) => {
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

function getGroupColorClass(groupName, groupItems) {
  if (!groupItems || groupItems.length === 0) {
    return 'neutral';
  }

  // Calculate average coverage for the group
  let totalCoverage = 0;
  let countWithCoverage = 0;
  let upCount = 0;
  let downCount = 0;
  let passedCount = 0;
  let failedCount = 0;

  for (const item of groupItems) {
    const current = Number(item.comparison?.currentTotalCoveragePercent);
    if (Number.isFinite(current)) {
      totalCoverage += current;
      countWithCoverage++;
    }

    const delta = Number(item.comparison?.deltaPercent);
    const threshold = item.comparison?.thresholdStatus;

    if (Number.isFinite(delta) && delta >= 0) {
      upCount++;
    } else if (Number.isFinite(delta) && delta < 0) {
      downCount++;
    }

    if (threshold === 'passed') {
      passedCount++;
    } else if (threshold === 'failed') {
      failedCount++;
    }
  }

  // Decision logic: Use threshold status if available, otherwise use delta trend
  if (passedCount > failedCount) {
    return 'up';
  } else if (failedCount > passedCount) {
    return 'down';
  } else if (upCount > downCount) {
    return 'up';
  } else if (downCount > upCount) {
    return 'down';
  }

  // Default based on average coverage
  const avgCoverage = countWithCoverage > 0 ? totalCoverage / countWithCoverage : 0;
  return avgCoverage >= 80 ? 'up' : 'down';
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

  // Group items by project group
  const grouped = {};
  const ungroupedItems = [];

  for (const item of heatmapItems) {
    const groupName = item.project.group || null;
    if (groupName) {
      if (!grouped[groupName]) {
        grouped[groupName] = [];
      }
      grouped[groupName].push(item);
    } else {
      ungroupedItems.push(item);
    }
  }

  // Sort items within each group by coverage
  const sortItems = (items) => {
    return items.sort((a, b) => {
      const ac = Number(a.comparison?.currentTotalCoveragePercent);
      const bc = Number(b.comparison?.currentTotalCoveragePercent);
      return (Number.isFinite(bc) ? bc : -1) - (Number.isFinite(ac) ? ac : -1);
    });
  };

  const sections = Object.keys(grouped)
    .sort()
    .map((groupName) => ({
      name: groupName,
      items: sortItems(grouped[groupName]),
    }));

  if (ungroupedItems.length > 0) {
    sections.push({
      name: 'Ungrouped',
      items: sortItems(ungroupedItems),
    });
  }

  for (const section of sections) {
    const groupSection = document.createElement('div');
    const colorClass = getGroupColorClass(section.name, section.items);
    groupSection.className = `heatmap-group heatmap-group-${colorClass}`;

    const groupHeader = document.createElement('h4');
    groupHeader.className = 'heatmap-group-header';
    groupHeader.textContent = section.name;
    groupSection.appendChild(groupHeader);

    const groupGrid = document.createElement('div');
    groupGrid.className = 'heatmap-group-grid';

    for (const item of section.items) {
      const tile = buildHeatmapTile(item, { compact: true });
      groupGrid.appendChild(tile);
    }

    groupSection.appendChild(groupGrid);
    heatmapGrid.appendChild(groupSection);
  }

  scheduleHeatmapLayout();
}

function buildHeatmapTile(item, options = {}) {
  const btn = document.createElement('button');
  const project = item.project;
  const name = project.name || project.projectKey;
  const current = Number(item.comparison?.currentTotalCoveragePercent);
  const delta = Number(item.comparison?.deltaPercent);
  const threshold = item.comparison?.thresholdStatus;
  const thresholdValue = Number(item.comparison?.thresholdPercent);
  const { compact = false } = options;

  const trendClass = heatTrendClass(current, delta, threshold);
  const size = tileSizeForCoverage(current);
  const classNames = ['heat-tile', trendClass];

  if (compact) {
    classNames.push('compact');
  }
  if (selectedProjectId === project.id) {
    classNames.push('selected');
  }

  btn.className = classNames.join(' ');

  if (compact) {
    btn.style.removeProperty('--col-span');
    btn.style.removeProperty('--row-span');
  } else {
    btn.style.setProperty('--col-span', String(size.col));
    btn.style.setProperty('--row-span', String(size.row));
  }

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

function scheduleHeatmapLayout() {
  if (heatmapLayoutFrame !== 0) {
    cancelAnimationFrame(heatmapLayoutFrame);
  }

  heatmapLayoutFrame = requestAnimationFrame(() => {
    heatmapLayoutFrame = 0;
    layoutHeatmapGroups();
  });
}

function layoutHeatmapGroups() {
  const groups = Array.from(heatmapGrid.querySelectorAll('.heatmap-group'));
  if (groups.length === 0) {
    heatmapGrid.style.removeProperty('--heatmap-group-columns');
    heatmapGrid.style.removeProperty('--heatmap-group-rows');
    return;
  }

  const width = heatmapGrid.clientWidth;
  const height = heatmapGrid.clientHeight;
  if (!width || !height) {
    return;
  }

  const groupLayout = fitGrid(groups.length, width / Math.max(height, 1));
  heatmapGrid.style.setProperty('--heatmap-group-columns', String(groupLayout.columns));
  heatmapGrid.style.setProperty('--heatmap-group-rows', String(groupLayout.rows));

  const remainder = groups.length % groupLayout.columns;

  groups.forEach((group, index) => {
    group.style.gridColumn = '';

    if (remainder !== 0 && index === groups.length - 1) {
      group.style.gridColumn = `span ${groupLayout.columns - remainder + 1}`;
    }

    const groupGrid = group.querySelector('.heatmap-group-grid');
    if (!groupGrid) {
      return;
    }

    const tiles = Array.from(groupGrid.querySelectorAll('.heat-tile'));
    const groupWidth = groupGrid.clientWidth || group.clientWidth;
    const groupHeight = groupGrid.clientHeight || group.clientHeight;
    const tileLayout = fitGrid(tiles.length, groupWidth / Math.max(groupHeight, 1));

    groupGrid.style.setProperty('--group-columns', String(tileLayout.columns));
    groupGrid.style.setProperty('--group-rows', String(tileLayout.rows));
  });
}

function fitGrid(itemCount, aspectRatio) {
  if (itemCount <= 1) {
    return { columns: 1, rows: 1 };
  }

  const safeAspectRatio = Number.isFinite(aspectRatio) && aspectRatio > 0 ? aspectRatio : 1;
  const columns = Math.max(1, Math.min(itemCount, Math.ceil(Math.sqrt(itemCount * safeAspectRatio))));
  const rows = Math.max(1, Math.ceil(itemCount / columns));
  return { columns, rows };
}

async function loadBranches(projectId) {
  try {
    const res = await fetch(`/api/projects/${projectId}/branches`);
    if (!res.ok) throw new Error(`failed to load branches (${res.status})`);
    const data = await res.json();
    availableBranches = data.branches || [];
    renderBranchSelector();
  } catch (err) {
    console.error('Error loading branches:', err);
    branchSelector.innerHTML = '<option value="">Error loading branches</option>';
  }
}

function renderBranchSelector() {
  branchSelector.innerHTML = '';
  
  // Add empty option for latest run
  const emptyOption = document.createElement('option');
  emptyOption.value = '';
  emptyOption.textContent = 'Latest Run (All Branches)';
  branchSelector.appendChild(emptyOption);
  
  // Add each branch as an option
  for (const branch of availableBranches) {
    const option = document.createElement('option');
    option.value = branch;
    option.textContent = branch;
    branchSelector.appendChild(option);
  }
  
  branchSelector.value = selectedBranch;
}

async function loadLatestComparison(projectId) {
  packagesBody.innerHTML = '';
  try {
    const url = new URL(`/api/projects/${projectId}/coverage-runs/latest-comparison`, window.location.origin);
    if (selectedBranch) {
      url.searchParams.set('branch', selectedBranch);
    }
    const res = await fetch(url.toString());
    if (!res.ok) throw new Error(`failed to load latest comparison (${res.status})`);
    const data = await res.json();

    currentCoverage.textContent = pct(data.comparison.currentTotalCoveragePercent);
    previousCoverage.textContent = data.comparison.previousTotalCoveragePercent == null ? '-' : pct(data.comparison.previousTotalCoveragePercent);
    deltaCoverage.textContent = data.comparison.deltaPercent == null ? '-' : signedPct(data.comparison.deltaPercent);
    thresholdPercent.textContent = pct(data.comparison.thresholdPercent);

    thresholdStatus.textContent = data.comparison.thresholdStatus || '-';
    thresholdStatus.className = `value ${data.comparison.thresholdStatus === 'passed' ? 'passed' : 'failed'}`;

    const project = projects.find((p) => p.id === projectId) || allProjects.find((p) => p.id === projectId);
    const defaultBranch = project?.defaultBranch || 'main';
    const runBranch = data.run?.branch || '-';
    const runType = data.run?.triggerType || 'unknown';
    const baselineSource = data.comparison?.baselineSource || 'latest_default_branch';
    const isPRComparison = runType === 'pr' && runBranch !== defaultBranch;

    compareSummary.textContent = isPRComparison
      ? `PR branch ${runBranch} is being compared against default branch ${defaultBranch}.`
      : `Latest ${runType.toUpperCase()} run on ${runBranch} is compared against default branch ${defaultBranch}.`;
    compareCurrent.textContent = runBranch;
    compareBaseline.textContent = `${defaultBranch} (${baselineSource})`;
    compareRunType.textContent = runType.toUpperCase();
    compareCard.classList.toggle('pr-mode', isPRComparison);

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
    compareSummary.textContent = err.message;
    compareCurrent.textContent = '-';
    compareBaseline.textContent = '-';
    compareRunType.textContent = '-';
    compareCard.classList.remove('pr-mode');

    const tr = document.createElement('tr');
    tr.innerHTML = `<td colspan="5" class="muted">${err.message}</td>`;
    packagesBody.appendChild(tr);
  }
}

async function loadTrendChart(projectId, defaultBranch) {
  const canvas = document.getElementById('trendChart');
  if (!canvas) return;

  try {
    const url = new URL(`/api/projects/${projectId}/coverage-runs`, window.location.origin);
    url.searchParams.set('branch', defaultBranch);
    url.searchParams.set('page', '1');
    url.searchParams.set('pageSize', '5');
    const res = await fetch(url.toString());
    if (!res.ok) throw new Error(`failed to load trend data (${res.status})`);
    const data = await res.json();

    // API returns newest first; reverse for left-to-right chronological order
    const runs = [...(data.items || [])].reverse();
    const labels = runs.map(r => r.commitSha.slice(0, 7));
    const values = runs.map(r => r.totalCoveragePercent);

    if (trendChart) {
      trendChart.destroy();
      trendChart = null;
    }

    if (values.length === 0) return;

    const minVal = Math.max(0, Math.min(...values) - 5);
    const maxVal = Math.min(100, Math.max(...values) + 5);

    trendChart = new Chart(canvas, {
      type: 'line',
      data: {
        labels,
        datasets: [{
          label: 'Coverage %',
          data: values,
          borderColor: '#14d8ff',
          backgroundColor: 'rgba(20, 216, 255, 0.08)',
          pointBackgroundColor: '#14d8ff',
          pointBorderColor: '#14d8ff',
          pointRadius: 5,
          pointHoverRadius: 7,
          tension: 0.3,
          fill: true,
        }],
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
          legend: { display: false },
          tooltip: {
            callbacks: {
              label: (ctx) => ` ${ctx.raw.toFixed(2)}%`,
            },
          },
        },
        scales: {
          x: {
            grid: { color: 'rgba(39, 52, 81, 0.5)' },
            ticks: { color: '#a4b2cf', font: { family: 'JetBrains Mono', size: 11 } },
          },
          y: {
            min: minVal,
            max: maxVal,
            grid: { color: 'rgba(39, 52, 81, 0.5)' },
            ticks: {
              color: '#a4b2cf',
              font: { family: 'Space Grotesk', size: 11 },
              callback: (v) => `${v}%`,
            },
          },
        },
      },
    });
  } catch (err) {
    console.error('Error loading trend chart:', err);
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
