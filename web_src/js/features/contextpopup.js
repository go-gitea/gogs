import {Label} from '../components.js';
import {SVG} from '../svg.js';

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
  $.get(`${AppSubUrl}/api/v1/repos/${owner}/${repo}/issues/${index}`, (issue) => {
    const createdAt = new Date(issue.created_at).toLocaleDateString(undefined, {year: 'numeric', month: 'short', day: 'numeric'});

    let body = issue.body.replace(/\n+/g, ' ');
    if (body.length > 85) {
      body = `${body.substring(0, 85)}...`;
    }

    let icon, color;
    if (issue.pull_request !== null) {
      if (issue.state === 'open') {
        color = 'green';
        icon = 'octicon-git-pull-request'; // Open PR
      } else if (issue.pull_request.merged === true) {
        color = 'purple';
        icon = 'octicon-git-merge'; // Merged PR
      } else {
        color = 'red';
        icon = 'octicon-git-pull-request'; // Closed PR
      }
    } else if (issue.state === 'open') {
      color = 'green';
      icon = 'octicon-issue-opened'; // Open Issue
    } else {
      color = 'red';
      icon = 'octicon-issue-closed'; // Closed Issue
    }

    $element.popup({
      variation: 'wide',
      delay: {
        show: 250
      },
      html: (
        <div>
          <p><small>{issue.repository.full_name} on {createdAt}</small></p>
          <p>
            <span class={color}><SVG name={icon}/></span>
            <strong class="mx-2">{issue.title}</strong>
            #{index}
          </p>
          <p>{body}</p>
          {issue.labels && issue.labels.length && (
            <p>
              {issue.labels.map((label) => (
                <Label label={label}/>
              ))}
            </p>
          )}
        </div>
      )
    });
  });
}
