---
date: "2016-12-01T16:00:00+02:00"
title: "Webhooks"
slug: "webhooks"
weight: 10
toc: false
draft: false
menu:
  sidebar:
    parent: "features"
    name: "Webhooks"
    weight: 30
    identifier: "webhooks"
---

# Webhooks

Gitea supports web hooks for repository events. This can be configured in the settings
page `/:username/:reponame/settings/hooks` by a repository admin. Webhooks can also be configured on a per-organization and whole system basis.
All event pushes are POST requests. The methods currently supported are:

- Gitea (can also be a GET request)
- Gogs
- Slack
- Discord
- Dingtalk
- Telegram
- Microsoft Teams
- Feishu
- Wechatwork

### Event information

**WARNING**: The `secret` field in the payload is deprecated as of Gitea 1.13.0 and will be removed in 1.14.0: https://github.com/go-gitea/gitea/issues/11755

The following is an example of event information that will be sent by Gitea to
a Payload URL:

```
X-GitHub-Delivery: f6266f16-1bf3-46a5-9ea4-602e06ead473
X-GitHub-Event: push
X-Gogs-Delivery: f6266f16-1bf3-46a5-9ea4-602e06ead473
X-Gogs-Event: push
X-Gitea-Delivery: f6266f16-1bf3-46a5-9ea4-602e06ead473
X-Gitea-Event: push
```

```json
{
  "secret": "3gEsCfjlV2ugRwgpU#w1*WaW*wa4NXgGmpCfkbG3",
  "ref": "refs/heads/develop",
  "before": "28e1879d029cb852e4844d9c718537df08844e03",
  "after": "bffeb74224043ba2feb48d137756c8a9331c449a",
  "compare_url": "http://localhost:3000/gitea/webhooks/compare/28e1879d029cb852e4844d9c718537df08844e03...bffeb74224043ba2feb48d137756c8a9331c449a",
  "commits": [
    {
      "id": "bffeb74224043ba2feb48d137756c8a9331c449a",
      "message": "Webhooks Yay!",
      "url": "http://localhost:3000/gitea/webhooks/commit/bffeb74224043ba2feb48d137756c8a9331c449a",
      "author": {
        "name": "Gitea",
        "email": "someone@gitea.io",
        "username": "gitea"
      },
      "committer": {
        "name": "Gitea",
        "email": "someone@gitea.io",
        "username": "gitea"
      },
      "timestamp": "2017-03-13T13:52:11-04:00"
    }
  ],
  "repository": {
    "id": 140,
    "owner": {
      "id": 1,
      "login": "gitea",
      "full_name": "Gitea",
      "email": "someone@gitea.io",
      "avatar_url": "https://localhost:3000/avatars/1",
      "username": "gitea"
    },
    "name": "webhooks",
    "full_name": "gitea/webhooks",
    "description": "",
    "private": false,
    "fork": false,
    "html_url": "http://localhost:3000/gitea/webhooks",
    "ssh_url": "ssh://gitea@localhost:2222/gitea/webhooks.git",
    "clone_url": "http://localhost:3000/gitea/webhooks.git",
    "website": "",
    "stars_count": 0,
    "forks_count": 1,
    "watchers_count": 1,
    "open_issues_count": 7,
    "default_branch": "master",
    "created_at": "2017-02-26T04:29:06-05:00",
    "updated_at": "2017-03-13T13:51:58-04:00"
  },
  "pusher": {
    "id": 1,
    "login": "gitea",
    "full_name": "Gitea",
    "email": "someone@gitea.io",
    "avatar_url": "https://localhost:3000/avatars/1",
    "username": "gitea"
  },
  "sender": {
    "id": 1,
    "login": "gitea",
    "full_name": "Gitea",
    "email": "someone@gitea.io",
    "avatar_url": "https://localhost:3000/avatars/1",
    "username": "gitea"
  }
}
```

### Example

This is an example of how to use webhooks to run a php script upon push requests to the repository.
In your repository Settings, under Webhooks, Setup a Gitea webhook as follows:

- Target URL: http://mydomain.com/webhook.php
- HTTP Method: POST
- POST Content Type: application/json
- Secret: 123
- Trigger On: Push Events
- Active: Checked

Now on your server create the php file webhook.php

```
<?php

$secret_key = '123';

// check for POST request
if ($_SERVER['REQUEST_METHOD'] != 'POST') {
    error_log('FAILED - not POST - '. $_SERVER['REQUEST_METHOD']);
    exit();
}

// get content type
$content_type = isset($_SERVER['CONTENT_TYPE']) ? strtolower(trim($_SERVER['CONTENT_TYPE'])) : '';

if ($content_type != 'application/json') {
    error_log('FAILED - not application/json - '. $content_type);
    exit();
}

// get payload
$payload = trim(file_get_contents("php://input"));

if (empty($payload)) {
    error_log('FAILED - no payload');
    exit();
}

// get header signature
$header_signature = isset($_SERVER['HTTP_X_GITEA_SIGNATURE']) ? $_SERVER['HTTP_X_GITEA_SIGNATURE'] : '';

if (empty($header_signature)) {
    error_log('FAILED - header signature missing');
    exit();
}

// calculate payload signature
$payload_signature = hash_hmac('sha256', $payload, $secret_key, false);

// check payload signature against header signature
if ($header_signature !== $payload_signature) {
    error_log('FAILED - payload signature');
    exit();
}

// convert json to array
$decoded = json_decode($payload, true);

// check for json decode errors
if (json_last_error() !== JSON_ERROR_NONE) {
    error_log('FAILED - json decode - '. json_last_error());
    exit();
}

// success, do something
```

There is a Test Delivery button in the webhook settings that allows to test the configuration as well as a list of the most Recent Deliveries.
