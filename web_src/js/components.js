import {contrastColor} from './utils.js';

// These components might look like React components but they are
// not. They return DOM nodes via JSX transformation using jsx-dom.
// https://github.com/proteriax/jsx-dom

export function Label({label}) {
  const backgroundColor = `#${label.color}`;
  const color = contrastColor(backgroundColor);
  const style = `color: ${color}; background-color: ${backgroundColor}`;

  return (
    <div class="ui label" style={style}>
      {label.name}
    </div>
  );
}
