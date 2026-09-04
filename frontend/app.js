const taskList = document.querySelector('#tasks');
const form = document.querySelector('#task-form');
const titleInput = document.querySelector('#title');
const statusEl = document.querySelector('#status');
const refreshButton = document.querySelector('#refresh');

async function api(path, options = {}) {
  const response = await fetch(path, {
    headers: { 'Content-Type': 'application/json', ...(options.headers || {}) },
    ...options,
  });

  if (!response.ok) {
    const body = await response.json().catch(() => ({ error: response.statusText }));
    throw new Error(body.error || 'Request failed');
  }

  if (response.status === 204) return null;
  return response.json();
}

function setStatus(message, isError = false) {
  statusEl.textContent = message;
  statusEl.classList.toggle('error', isError);
}

function escapeHtml(value) {
  const el = document.createElement('div');
  el.textContent = value;
  return el.innerHTML;
}

function renderTasks(tasks) {
  taskList.innerHTML = '';
  if (!tasks.length) {
    taskList.innerHTML = '<li class="empty">No tasks yet. Add one above.</li>';
    return;
  }

  for (const task of tasks) {
    const item = document.createElement('li');
    item.className = `task ${task.completed ? 'done' : ''}`;
    item.innerHTML = `
      <label>
        <input type="checkbox" ${task.completed ? 'checked' : ''} data-id="${task.id}" />
        <span>${escapeHtml(task.title)}</span>
      </label>
      <button class="danger" data-delete="${task.id}">Delete</button>
    `;
    taskList.appendChild(item);
  }
}

async function loadTasks() {
  try {
    const tasks = await api('/api/tasks');
    renderTasks(tasks);
    setStatus(`Loaded ${tasks.length} task${tasks.length === 1 ? '' : 's'}.`);
  } catch (error) {
    setStatus(error.message, true);
  }
}

form.addEventListener('submit', async (event) => {
  event.preventDefault();
  const title = titleInput.value.trim();
  if (!title) return;

  try {
    await api('/api/tasks', {
      method: 'POST',
      body: JSON.stringify({ title }),
    });
    titleInput.value = '';
    await loadTasks();
  } catch (error) {
    setStatus(error.message, true);
  }
});

taskList.addEventListener('change', async (event) => {
  const checkbox = event.target.closest('input[type="checkbox"][data-id]');
  if (!checkbox) return;

  try {
    await api(`/api/tasks/${checkbox.dataset.id}`, {
      method: 'PATCH',
      body: JSON.stringify({ completed: checkbox.checked }),
    });
    await loadTasks();
  } catch (error) {
    setStatus(error.message, true);
  }
});

taskList.addEventListener('click', async (event) => {
  const button = event.target.closest('button[data-delete]');
  if (!button) return;

  try {
    await api(`/api/tasks/${button.dataset.delete}`, { method: 'DELETE' });
    await loadTasks();
  } catch (error) {
    setStatus(error.message, true);
  }
});

refreshButton.addEventListener('click', loadTasks);
loadTasks();
