const {AppSubUrl, csrf, NotificationSettings} = window.config;

let notificationSequenceNumber = 0;

export function initNotificationsTable() {
  $('#notification_table .button').on('click', async function () {
    const data = await updateNotification(
      $(this).data('url'),
      $(this).data('status'),
      $(this).data('page'),
      $(this).data('q'),
      $(this).data('notification-id'),
    );

    if ($(data).data('sequence-number') === notificationSequenceNumber) {
      $('#notification_div').replaceWith(data);
      initNotificationsTable();
    }
    await updateNotificationCount();

    return false;
  });
}

async function receiveUpdateCount(event) {
  try {
    const data = JSON.parse(event.data);

    const notificationCount = document.querySelector('.notification_count');
    if (data.Count > 0) {
      notificationCount.classList.remove('hidden');
    } else {
      notificationCount.classList.add('hidden');
    }

    notificationCount.textContent = `${data.Count}`;
    await updateNotificationTable();
  } catch (error) {
    console.error(error, event);
  }
}

export async function initNotificationCount() {
  const notificationCount = $('.notification_count');

  if (!notificationCount.length) {
    return;
  }

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
      if (event.data.type === 'notification-count') {
        receiveUpdateCount(event.data);
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

  const fn = (timeout, lastCount) => {
    setTimeout(async () => {
      await updateNotificationCountWithCallback(fn, timeout, lastCount);
    }, timeout);
  };

  fn(NotificationSettings.MinTimeout, notificationCount.text());
}

async function updateNotificationCountWithCallback(callback, timeout, lastCount) {
  const currentCount = $('.notification_count').text();
  if (lastCount !== currentCount) {
    callback(NotificationSettings.MinTimeout, currentCount);
    return;
  }

  const newCount = await updateNotificationCount();
  let needsUpdate = false;

  if (lastCount !== newCount) {
    needsUpdate = true;
    timeout = NotificationSettings.MinTimeout;
  } else if (timeout < NotificationSettings.MaxTimeout) {
    timeout += NotificationSettings.TimeoutStep;
  }

  callback(timeout, newCount);
  if (needsUpdate) {
    await updateNotificationTable();
  }
}

async function updateNotificationTable() {
  const notificationDiv = $('#notification_div');
  if (notificationDiv.length > 0) {
    const data = await $.ajax({
      type: 'GET',
      url: `${AppSubUrl}/notifications?${notificationDiv.data('params')}`,
      data: {
        'div-only': true,
        'sequence-number': ++notificationSequenceNumber,
      }
    });
    if ($(data).data('sequence-number') === notificationSequenceNumber) {
      notificationDiv.replaceWith(data);
      initNotificationsTable();
    }
  }
}

async function updateNotificationCount() {
  const data = await $.ajax({
    type: 'GET',
    url: `${AppSubUrl}/api/v1/notifications/new`,
    headers: {
      'X-Csrf-Token': csrf,
    },
  });

  const notificationCount = $('.notification_count');
  if (data.new === 0) {
    notificationCount.addClass('hidden');
  } else {
    notificationCount.removeClass('hidden');
  }

  notificationCount.text(`${data.new}`);

  return `${data.new}`;
}

async function updateNotification(url, status, page, q, notificationID) {
  if (status !== 'pinned') {
    $(`#notification_${notificationID}`).remove();
  }

  return $.ajax({
    type: 'POST',
    url,
    data: {
      _csrf: csrf,
      notification_id: notificationID,
      status,
      page,
      q,
      noredirect: true,
      'sequence-number': ++notificationSequenceNumber,
    },
  });
}
