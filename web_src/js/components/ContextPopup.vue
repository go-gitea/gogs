<template>
  <div>
    <div v-if="loading" class="ui active centered inline loader"/>
    <div v-if="!loading && issue !== null">
      <p><small>{{ issue.repository.full_name }} on {{ createdAt }}</small></p>
      <p><svg-icon :name="icon" :class="[color]" /> <strong>{{ issue.title }}</strong> #{{ issue.number }}</p>
      <p>{{ body }}</p>
      <div>
        <div
          v-for="label in labels"
          :key="label.name"
          class="ui label"
          :style="{ color: label.textColor, backgroundColor: label.color }"
        >
          {{ label.name }}
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import {SvgIcon} from '../svg.js';

const {AppSubUrl} = window.config;

export default {
  name: 'ContextPopup',

  components: {
    SvgIcon,
  },

  data: () => ({
    loading: false,
    issue: null
  }),

  computed: {
    createdAt() {
      return new Date(this.issue.created_at).toLocaleDateString(undefined, {year: 'numeric', month: 'short', day: 'numeric'});
    },

    body() {
      const body = this.issue.body.replace(/\n+/g, ' ');
      if (body.length > 85) {
        return `${body.substring(0, 85)}…`;
      }
      return body;
    },

    icon() {
      if (this.issue.pull_request !== null) {
        if (this.issue.state === 'open') {
          return 'octicon-git-pull-request'; // Open PR
        } else if (this.issue.pull_request.merged === true) {
          return 'octicon-git-merge'; // Merged PR
        }
        return 'octicon-git-pull-request'; // Closed PR
      } else if (this.issue.state === 'open') {
        return 'octicon-issue-opened'; // Open Issue
      }
      return 'octicon-issue-closed'; // Closed Issue
    },

    color() {
      if (this.issue.state === 'open') {
        return 'green';
      } else if (this.issue.pull_request !== null && this.issue.pull_request.merged === true) {
        return 'purple';
      }
      return 'red';
    },

    labels() {
      return this.issue.labels.map((label) => {
        const red = parseInt(label.color.substring(0, 2), 16);
        const green = parseInt(label.color.substring(2, 4), 16);
        const blue = parseInt(label.color.substring(4, 6), 16);
        let color = '#ffffff';
        if ((red * 0.299 + green * 0.587 + blue * 0.114) > 125) {
          color = '#000000';
        }
        return {name: label.name, color: `#${label.color}`, textColor: color};
      });
    }
  },

  mounted() {
    this.$root.$on('load-context-popup', (data, callback) => {
      if (!this.loading && this.issue === null) {
        this.load(data, callback);
      }
    });
  },

  methods: {
    load(data, callback) {
      this.loading = true;
      $.get(`${AppSubUrl}/api/v1/repos/${data.owner}/${data.repo}/issues/${data.index}`, (issue) => {
        this.issue = issue;
        this.loading = false;
        this.$nextTick(() => {
          if (callback) {
            callback();
          }
        });
      });
    }
  }
};
</script>
