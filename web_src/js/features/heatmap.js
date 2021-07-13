import Vue from 'vue';

import ActivityHeatmap from '../components/ActivityHeatmap.vue';

export default async function initHeatmap() {
  const el = document.getElementById('user-heatmap');
  if (!el) return;

  try {
    const heatmap = {};
    JSON.parse(el.dataset.heatmapData).forEach(({contributions, timestamp}) => {
      // Convert to user timezone and sum contributions by date
      const dateStr = new Date(timestamp * 1000).toDateString();
      heatmap[dateStr] = (heatmap[dateStr] || 0) + contributions;
    });

    const values = Object.keys(heatmap).map((v) => {
      return {date: new Date(v), count: heatmap[v]};
    });

    const View = Vue.extend({
      render: (createElement) => createElement(ActivityHeatmap, {props: {values}}),
    });

    new View().$mount(el);
  } catch (err) {
    console.error(err);
    el.textContent = 'Heatmap failed to load';
  }
}
