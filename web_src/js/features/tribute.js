import {emojiKeys, emojiHTML, emojiString} from './emoji.js';
import {uniq} from '../utils.js';

function makeCollections({mentions, emoji}) {
  const collections = [];

  if (mentions) {
    collections.push({
      trigger: ':',
      requireLeadingSpace: true,
      values: (query, cb) => {
        const matches = [];
        for (const name of emojiKeys) {
          if (name.includes(query)) {
            matches.push(name);
            if (matches.length > 5) break;
          }
        }
        cb(matches);
      },
      lookup: (item) => item,
      selectTemplate: (item) => {
        if (typeof item === 'undefined') return null;
        return emojiString(item.original);
      },
      menuItemTemplate: (item) => {
        return `<div class="tribute-item">${emojiHTML(item.original)}<span>${item.original}</span></div>`;
      }
    });
  }

  if (emoji) {
    collections.push({
      values: window.config.tributeValues,
      requireLeadingSpace: true,
      menuItemTemplate: (item) => {
        return `
          <div class="tribute-item">
            <img src="${item.original.avatar}"/>
            <span class="name">${item.original.name}</span>
            ${item.original.fullname && item.original.fullname !== '' ? `<span class="fullname">${item.original.fullname}</span>` : ''}
          </div>
        `;
      }
    });
  }

  return collections;
}

export default async function attachTribute(elementOrNodeList, {mentions, emoji} = {}) {
  if (!window.config.Tribute || !elementOrNodeList) return;
  const nodes = Array.from('length' in elementOrNodeList ? elementOrNodeList : [elementOrNodeList]);
  if (!nodes.length) return;

  const mentionNodes = nodes.filter((node) => {
    return mentions || node.id === 'content';
  });
  const emojiNodes = nodes.filter((node) => {
    return emoji || node.id === 'content' || node.classList.contains('emoji-input');
  });
  const uniqueNodes = uniq([...mentionNodes, ...emojiNodes]);
  if (!uniqueNodes.length) return;

  const {default: Tribute} = await import(/* webpackChunkName: "tribute" */'tributejs');

  const collections = makeCollections({
    mentions: mentions || mentionNodes.length > 0,
    emoji: emoji || emojiNodes.length > 0,
  });

  const tribute = new Tribute({collection: collections, noMatchTemplate: ''});
  for (const node of uniqueNodes) {
    tribute.attach(node);
  }
  return tribute;
}
