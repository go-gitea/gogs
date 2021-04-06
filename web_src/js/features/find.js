import {svg} from '../svg.js';
const {AppSubUrl, csrf} = window.config;

const threshold = 50;
let files = [];

function hitAllKeys(keys, entry) {
  let i = 0;
  let j = 0;
  const hitIndexes = [];
  while (i < keys.length) {
    for (; j < entry.length; j++) {
      if (keys[i] === entry[j]) {
        hitIndexes.push(j);
        j++;
        break;
      }
    }
    i++;
  }

  return hitIndexes;
}

function addHighLightToHit(entry, indexes) {
  let highLightText = '';
  for (let i = 0; i < entry.length; i++) {
    if (indexes.includes(i)) {
      highLightText += `<b>${entry[i]}</b>`;
    } else {
      highLightText += entry[i];
    }
  }
  return highLightText;
}

// function removeHighLight($a, entry) {
//   $a.text(entry.replace(/(<([^>]+)>)/ig, ''));
// }

function filterRepoFiles(keys) {
  if (keys.length > 0) {
    // Remove all tr
    $('#repo-find-files-table tbody').empty();

    let hit = false;
    const treeLink = $('#treeLink').val();
    for (let i = 0; i < files.length; i++) {
      if (i >= threshold) break;

      const keysTrim = keys.trim();
      const hitIndexes = hitAllKeys(keysTrim, files[i]);
      if (hitIndexes.length > 0 && keysTrim.length === hitIndexes.length) {
        const textWithHl = addHighLightToHit(files[i], hitIndexes);
        generateTrWithHighlight(treeLink, files[i], textWithHl);
        hit = true;
      }
    }
    if (hit) {
      $('#no-hit-prompt').hide();
    } else {
      $('#no-hit-prompt').show();
    }
  } else {
    // Remove all tr
    $('#repo-find-files-table tbody').empty();
    $('#no-hit-prompt').hide();

    loadDefaultDataByFiles();
  }
}

function generateTrWithHighlight(treeLink, filename, filenameWithHl) {
  // Generate new tr
  const tr_wrap = $('<tr>');

  const td_wrap = $('<td>', {
    class: 'name four wide'
  }).appendTo(tr_wrap);

  const span_wrap = $('<span>', {
    class: 'truncate'
  }).appendTo(td_wrap);

  const div_wrap = $('<div>').append(svg('octicon-file')).appendTo(span_wrap);

  $('<a>', {
    class: 'find-file-name ml-2',
    title: filename,
  }).attr('href', `${treeLink}/${filename}`)
    .html(filenameWithHl)
    .appendTo(div_wrap);

  $('#repo-find-files-table tbody').append(tr_wrap);
}

function generateTr(treeLink, filename) {
  // Generate new tr
  const tr_wrap = $('<tr>');

  const td_wrap = $('<td>', {
    class: 'name four wide'
  }).appendTo(tr_wrap);

  const span_wrap = $('<span>', {
    class: 'truncate'
  }).appendTo(td_wrap);

  const div_wrap = $('<div>').append(svg('octicon-file')).appendTo(span_wrap);

  $('<a>', {
    class: 'find-file-name ml-2',
    title: filename,
  }).attr('href', `${treeLink}/${filename}`)
    .text(filename)
    .appendTo(div_wrap);

  $('#repo-find-files-table tbody').append(tr_wrap);
}

function loadDefaultDataByFiles() {
  const treeLink = $('#treeLink').val();

  for (let i = 0; i < files.length; i++) {
    if (i >= threshold) break;

    generateTr(treeLink, files[i]);
  }
}

async function fetchRepoFiles() {
  const ownerName = $('#ownerName').val();
  const repoName = $('#repoName').val();
  const branchName = $('#branchName').val();
  const data = await $.ajax({
    type: 'GET',
    url: `${AppSubUrl}/api/v1/repos/${ownerName}/${repoName}/find/${branchName}`,
    headers: {'X-Csrf-Token': csrf}
  });
  if (data) {
    files = data;
    loadDefaultDataByFiles();
  }
}

export default async function initFindFileInRepo() {
  const findContainer = document.getElementById('repo-file-find-container');
  if (!findContainer) return;

  await fetchRepoFiles();
  $('#repo-file-find-input').on('change paste keyup', () => {
    filterRepoFiles($('#repo-file-find-input').val());
  });
}
