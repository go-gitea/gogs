import Vue from 'vue';

import ContextPopup from '../components/ContextPopup.vue';

export default function initContextPopups() {
  const refIssues = $('.ref-issue');
  if (!refIssues.length) return;

  refIssues.each(function () {
    const [index, _issues, repo, owner] = $(this).attr('href').replace(/[#?].*$/, '').split('/').reverse();

    const el = document.createElement('div');
    el.className = 'ui custom popup hidden';
    el.innerHTML = '<div></div>';
    this.parentNode.insertBefore(el, this.nextSibling);

    const View = Vue.extend({
      render: (createElement) => createElement(ContextPopup),
    });

    const view = new View();

    try {
      view.$mount(el.firstChild);
    } catch (err) {
      console.error(err);
      el.textContent = 'ContextPopup failed to load';
    }

    $(this).popup({
      variation: 'wide',
      delay: {
        show: 250
      },
      onShow: () => {
        view.$emit('load-context-popup', {owner, repo, index}, () => {
          $(this).popup('reposition');
        });
      },
      popup: $(el),
    });
  });
}
