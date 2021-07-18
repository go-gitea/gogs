const {csrf} = window.config;

export async function initLastCommitLoader() {
  const entryMap = {};

  const entries = $('table#repo-files-table tr.notready')
    .map((_, v) => {
      entryMap[$(v).data('entryname')] = $(v);
      return $(v).data('entryname');
    })
    .get();

  if (entries.length === 0) {
    return;
  }

  const lastCommitLoaderURL = $('table#repo-files-table').data('lastCommitLoaderUrl');

  $.post(lastCommitLoaderURL, {
    _csrf: csrf,
    'f': entries,
  }, (data) => {
    $(data).find('tr').each((_, row) => {
      if (row.className === 'commit-list') {
        $('table#repo-files-table .commit-list').replaceWith(row);
        return;
      }
      entryMap[$(row).data('entryname')].replaceWith(row);
    });
  });
}
