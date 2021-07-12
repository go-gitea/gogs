import {htmlEscape} from 'escape-goat';
import {svg} from '../svg.js';

const {AppSubUrl} = window.config;

export default function initContextPopups() {
  const refIssues = $('.ref-issue');
  if (!refIssues.length) return;

  refIssues.each(function () {
    const [index, _issues, repo, owner] = $(this).attr('href').replace(/[#?].*$/, '').split('/').reverse();
    issuePopup(owner, repo, index, $(this));
  });
}

function issuePopup(owner, repo, index, $element) {
  $.get(`${AppSubUrl}/api/ui/repos/${owner}/${repo}/issues/${index}`, (issue) => {
    const createdAt = new Date(issue.created_at).toLocaleDateString(undefined, {year: 'numeric', month: 'short', day: 'numeric'});

    let body = issue.body.replace(/\n+/g, ' ');
    if (body.length > 85) {
      body = `${body.substring(0, 85)}...`;
    }

    let labels = '';
    for (let i = 0; i < issue.labels.length; i++) {
      const label = issue.labels[i];
      const red = parseInt(label.color.substring(0, 2), 16);
      const green = parseInt(label.color.substring(2, 4), 16);
      const blue = parseInt(label.color.substring(4, 6), 16);
      let color = '#ffffff';
      if ((red * 0.299 + green * 0.587 + blue * 0.114) > 125) {
        color = '#000000';
      }
      labels += `<div class="ui label" style="color: ${color}; background-color:#${label.color};">${htmlEscape(label.name)}</div>`;
    }
    if (labels.length > 0) {
      labels = `<p>${labels}</p>`;
    }

    let octicon, color;
    if (issue.pull_request !== null) {
      if (issue.state === 'open') {
        color = 'green';
        octicon = 'octicon-git-pull-request'; // Open PR
      } else if (issue.pull_request.merged === true) {
        color = 'purple';
        octicon = 'octicon-git-merge'; // Merged PR
      } else {
        color = 'red';
        octicon = 'octicon-git-pull-request'; // Closed PR
      }
    } else if (issue.state === 'open') {
      color = 'green';
      octicon = 'octicon-issue-opened'; // Open Issue
    } else {
      color = 'red';
      octicon = 'octicon-issue-closed'; // Closed Issue
    }

    $element.popup({
      variation: 'wide',
      delay: {
        show: 250
      },
      html: `
<div>
  <p><small>${htmlEscape(issue.repository.full_name)} on ${createdAt}</small></p>
  <p><span class="${color}">${svg(octicon)}</span> <strong>${htmlEscape(issue.title)}</strong> #${index}</p>
  <p>${htmlEscape(body)}</p>
  ${labels}
</div>
`
    });
  });
}
