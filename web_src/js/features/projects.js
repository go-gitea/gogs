const {csrf, PageIsProjects} = window.config;

export default async function initProject() {
  if (!PageIsProjects) {
    return;
  }

  const {Sortable} = await import(/* webpackChunkName: "sortable" */'sortablejs');
  const boardColumns = document.getElementsByClassName('board-column');

  new Sortable(
    document.getElementsByClassName('board')[0],
    {
      group: 'board-column',
      draggable: '.board-column',
      animation: 150,
      onSort: () => {
        const board = document.getElementsByClassName('board')[0];
        const boardColumns = board.getElementsByClassName('board-column');

        boardColumns.forEach((column, i) => {
          if (parseInt($(column).data('sorting')) !== i) {
            $.ajax({
              url: $(column).data('url'),
              data: JSON.stringify({sorting: i}),
              headers: {
                'X-Csrf-Token': csrf,
                'X-Remote': true,
              },
              contentType: 'application/json',
              method: 'PUT',
            });
          }
        });
      },
    },
  );

  for (const column of boardColumns) {
    new Sortable(
      column.getElementsByClassName('board')[0],
      {
        group: 'shared',
        animation: 150,
        onAdd: (e) => {
          $.ajax(`${e.to.dataset.url}/${e.item.dataset.issue}`, {
            headers: {
              'X-Csrf-Token': csrf,
              'X-Remote': true,
            },
            contentType: 'application/json',
            type: 'POST',
            success: () => {
              // Update the number of cards in board while drop successful.
              const fromCnt = e.from.parentElement.getElementsByClassName('board-card').length;
              e.from.parentElement.getElementsByClassName('board-card-cnt')[0].innerText = fromCnt;

              const toCnt = e.to.parentElement.getElementsByClassName('board-card').length;
              e.to.parentElement.getElementsByClassName('board-card-cnt')[0].innerText = toCnt;
            },
            error: () => {
              e.from.insertBefore(e.item, e.from.children[e.oldIndex]);
            },
          });
        },
      },
    );
  }

  $('.edit-project-board').each(function () {
    const projectTitleLabel = $(this).closest('.board-column-header').find('.board-label');
    const projectTitleInput = $(this).find(
      '.content > .form > .field > .project-board-title',
    );

    $(this)
      .find('.content > .form > .actions > .red')
      .on('click', function (e) {
        e.preventDefault();

        $.ajax({
          url: $(this).data('url'),
          data: JSON.stringify({title: projectTitleInput.val()}),
          headers: {
            'X-Csrf-Token': csrf,
            'X-Remote': true,
          },
          contentType: 'application/json',
          method: 'PUT',
        }).done(() => {
          projectTitleLabel.text(projectTitleInput.val());
          projectTitleInput.closest('form').removeClass('dirty');
          $('.ui.modal').modal('hide');
        });
      });
  });

  $(document).on('click', '.set-default-project-board', async function (e) {
    e.preventDefault();

    await $.ajax({
      method: 'POST',
      url: $(this).data('url'),
      headers: {
        'X-Csrf-Token': csrf,
        'X-Remote': true,
      },
      contentType: 'application/json',
    });

    window.location.reload();
  });

  $('.delete-project-board').each(function () {
    $(this).click(function (e) {
      e.preventDefault();

      $.ajax({
        url: $(this).data('url'),
        headers: {
          'X-Csrf-Token': csrf,
          'X-Remote': true,
        },
        contentType: 'application/json',
        method: 'DELETE',
      }).done(() => {
        window.location.reload();
      });
    });
  });

  $('#new_board_submit').click(function (e) {
    e.preventDefault();

    const boardTitle = $('#new_board');

    $.ajax({
      url: $(this).data('url'),
      data: JSON.stringify({title: boardTitle.val()}),
      headers: {
        'X-Csrf-Token': csrf,
        'X-Remote': true,
      },
      contentType: 'application/json',
      method: 'POST',
    }).done(() => {
      boardTitle.closest('form').removeClass('dirty');
      window.location.reload();
    });
  });
}
