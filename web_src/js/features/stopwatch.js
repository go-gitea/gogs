import prettyMilliseconds from 'pretty-ms';
const {AppSubUrl, csrf, NotificationSettings, EnableTimetracking} = window.config;

let updateTimeInterval = null; // holds setInterval id when active

export async function initStopwatch() {
  if (!EnableTimetracking) {
    return;
  }

  const stopwatchEl = $('.active-stopwatch-trigger');

  if (!stopwatchEl.length) {
    return;
  }

  stopwatchEl.removeAttr('href'); // intended for noscript mode only
  stopwatchEl.popup({
    position: 'bottom right',
    hoverable: true,
  });

  // form handlers
  $('form > button', stopwatchEl).on('click', function () {
    $(this).parent().trigger('submit');
  });

  if (NotificationSettings.EventSourceUpdateTime > 0 && !!window.EventSource && window.SharedWorker) {
    // Try to connect to the event source via the shared worker first
    const worker = new SharedWorker(`${__webpack_public_path__}js/eventsource.sharedworker.js`, 'notification-worker');
    worker.addEventListener('error', (event) => {
      console.error(event);
    });
    worker.port.onmessageerror = () => {
      console.error('Unable to deserialize message');
    };
    worker.port.postMessage({
      type: 'start',
      url: `${window.location.origin}${AppSubUrl}/user/events`,
    });
    worker.port.addEventListener('message', (event) => {
      if (!event.data || !event.data.type) {
        console.error(event);
        return;
      }
      if (event.data.type === 'stopwatches') {
        updateStopwatchData(JSON.parse(event.data.data));
      } else if (event.data.type === 'error') {
        console.error(event.data);
      } else if (event.data.type === 'logout') {
        if (event.data.data !== 'here') {
          return;
        }
        worker.port.postMessage({
          type: 'close',
        });
        worker.port.close();
        window.location.href = AppSubUrl;
      } else if (event.data.type === 'close') {
        worker.port.postMessage({
          type: 'close',
        });
        worker.port.close();
      }
    });
    worker.port.addEventListener('error', (e) => {
      console.error(e);
    });
    worker.port.start();
    window.addEventListener('beforeunload', () => {
      worker.port.postMessage({
        type: 'close',
      });
      worker.port.close();
    });

    return;
  }

  if (NotificationSettings.MinTimeout <= 0) {
    return;
  }

  const fn = (timeout) => {
    setTimeout(async () => {
      await updateStopwatchWithCallback(fn, timeout);
    }, timeout);
  };

  fn(NotificationSettings.MinTimeout);

  const currSeconds = $('.stopwatch-time').data('seconds');
  if (currSeconds) {
    updateTimeInterval = updateStopwatchTime(currSeconds);
  }
}

async function updateStopwatchWithCallback(callback, timeout) {
  const isSet = await updateStopwatch();

  if (!isSet) {
    timeout = NotificationSettings.MinTimeout;
  } else if (timeout < NotificationSettings.MaxTimeout) {
    timeout += NotificationSettings.TimeoutStep;
  }

  callback(timeout);
}

async function updateStopwatch() {
  const data = await $.ajax({
    type: 'GET',
    url: `${AppSubUrl}/api/ui/user/stopwatches`,
    headers: {'X-Csrf-Token': csrf},
  });

  if (updateTimeInterval) {
    clearInterval(updateTimeInterval);
    updateTimeInterval = null;
  }

  return updateStopwatchData(data);
}

async function updateStopwatchData(data) {
  const watch = data[0];
  const btnEl = $('.active-stopwatch-trigger');
  if (!watch) {
    btnEl.addClass('hidden');
  } else {
    const {repo_owner_name, repo_name, issue_index, seconds} = watch;
    const issueUrl = `${AppSubUrl}/${repo_owner_name}/${repo_name}/issues/${issue_index}`;
    $('.stopwatch-link').attr('href', issueUrl);
    $('.stopwatch-commit').attr('action', `${issueUrl}/times/stopwatch/toggle`);
    $('.stopwatch-cancel').attr('action', `${issueUrl}/times/stopwatch/cancel`);
    $('.stopwatch-issue').text(`${repo_owner_name}/${repo_name}#${issue_index}`);
    $('.stopwatch-time').text(prettyMilliseconds(seconds * 1000));
    updateStopwatchTime(seconds);
    btnEl.removeClass('hidden');
  }

  return !!data.length;
}

async function updateStopwatchTime(seconds) {
  const secs = parseInt(seconds);
  if (!Number.isFinite(secs)) return;

  const start = Date.now();
  updateTimeInterval = setInterval(() => {
    const delta = Date.now() - start;
    const dur = prettyMilliseconds(secs * 1000 + delta, {compact: true});
    $('.stopwatch-time').text(dur);
  }, 1000);
}
