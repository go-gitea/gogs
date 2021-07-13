import {
  basename, extname, isObject, uniq, stripTags, joinPaths, contrastColor,
} from './utils.js';

test('basename', () => {
  expect(basename('/path/to/file.js')).toEqual('file.js');
  expect(basename('/path/to/file')).toEqual('file');
  expect(basename('file.js')).toEqual('file.js');
});

test('extname', () => {
  expect(extname('/path/to/file.js')).toEqual('.js');
  expect(extname('/path/')).toEqual('');
  expect(extname('/path')).toEqual('');
  expect(extname('file.js')).toEqual('.js');
});

test('joinPaths', () => {
  expect(joinPaths('', '')).toEqual('');
  expect(joinPaths('', 'b')).toEqual('b');
  expect(joinPaths('', '/b')).toEqual('/b');
  expect(joinPaths('', '/b/')).toEqual('/b/');
  expect(joinPaths('a', '')).toEqual('a');
  expect(joinPaths('/a', '')).toEqual('/a');
  expect(joinPaths('/a/', '')).toEqual('/a/');
  expect(joinPaths('a', 'b')).toEqual('a/b');
  expect(joinPaths('a', '/b')).toEqual('a/b');
  expect(joinPaths('/a', '/b')).toEqual('/a/b');
  expect(joinPaths('/a', '/b')).toEqual('/a/b');
  expect(joinPaths('/a/', '/b')).toEqual('/a/b');
  expect(joinPaths('/a', '/b/')).toEqual('/a/b/');
  expect(joinPaths('/a/', '/b/')).toEqual('/a/b/');

  expect(joinPaths('', '', '')).toEqual('');
  expect(joinPaths('', 'b', '')).toEqual('b');
  expect(joinPaths('', 'b', 'c')).toEqual('b/c');
  expect(joinPaths('', '', 'c')).toEqual('c');
  expect(joinPaths('', '/b', '/c')).toEqual('/b/c');
  expect(joinPaths('/a', '', '/c')).toEqual('/a/c');
  expect(joinPaths('/a', '/b', '')).toEqual('/a/b');

  expect(joinPaths('', '/')).toEqual('/');
  expect(joinPaths('a', '/')).toEqual('a/');
  expect(joinPaths('', '/', '/')).toEqual('/');
  expect(joinPaths('/', '/')).toEqual('/');
  expect(joinPaths('/', '')).toEqual('/');
  expect(joinPaths('/', 'b')).toEqual('/b');
  expect(joinPaths('/', 'b/')).toEqual('/b/');
  expect(joinPaths('/', '', '/')).toEqual('/');
  expect(joinPaths('/', 'b', '/')).toEqual('/b/');
  expect(joinPaths('/', 'b/', '/')).toEqual('/b/');
  expect(joinPaths('a', '/', '/')).toEqual('a/');
  expect(joinPaths('/', '/', 'c')).toEqual('/c');
  expect(joinPaths('/', '/', 'c/')).toEqual('/c/');
});

test('isObject', () => {
  expect(isObject({})).toBeTrue();
  expect(isObject([])).toBeFalse();
});

test('uniq', () => {
  expect(uniq([1, 1, 1, 2])).toEqual([1, 2]);
});

test('stripTags', () => {
  expect(stripTags('<a>test</a>')).toEqual('test');
});

test('contrastColor', () => {
  expect(contrastColor('#000000')).toEqual('#fff');
  expect(contrastColor('#333333')).toEqual('#fff');
  expect(contrastColor('#ff0000')).toEqual('#fff');
  expect(contrastColor('#0000ff')).toEqual('#fff');
  expect(contrastColor('#cccccc')).toEqual('#000');
  expect(contrastColor('#ffffff')).toEqual('#000');
  expect(contrastColor('000000')).toEqual('#fff');
  expect(contrastColor('ffffff')).toEqual('#000');
});
