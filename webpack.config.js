import fastGlob from 'fast-glob';
import wrapAnsi from 'wrap-ansi';
import AddAssetPlugin from 'add-asset-webpack-plugin';
import LicenseCheckerWebpackPlugin from 'license-checker-webpack-plugin';
import MiniCssExtractPlugin from 'mini-css-extract-plugin';
import MonacoWebpackPlugin from 'monaco-editor-webpack-plugin';
import VueLoader from 'vue-loader';
import EsBuildLoader from 'esbuild-loader';
import {resolve, parse, dirname} from 'path';
import webpack from 'webpack';
import {fileURLToPath} from 'url';

const {VueLoaderPlugin} = VueLoader;
const {ESBuildMinifyPlugin} = EsBuildLoader;
const {SourceMapDevToolPlugin} = webpack;
const __dirname = dirname(fileURLToPath(import.meta.url));
const glob = (pattern) => fastGlob.sync(pattern, {cwd: __dirname, absolute: true});

const themes = {};
for (const path of glob('web_src/less/themes/*.less')) {
  themes[parse(path).name] = [path];
}

const isProduction = process.env.NODE_ENV !== 'development';

const filterCssImport = (url, ...args) => {
  const cssFile = args[1] || args[0]; // resourcePath is 2nd argument for url and 3rd for import
  const importedFile = url.replace(/[?#].+/, '').toLowerCase();

  if (cssFile.includes('fomantic')) {
    if (/brand-icons/.test(importedFile)) return false;
    if (/(eot|ttf|otf|woff|svg)$/.test(importedFile)) return false;
  }

  if (cssFile.includes('font-awesome') && /(eot|ttf|otf|woff|svg)$/.test(importedFile)) {
    return false;
  }

  return true;
};

export default {
  mode: isProduction ? 'production' : 'development',
  entry: {
    index: [
      resolve(__dirname, 'web_src/js/jquery.js'),
      resolve(__dirname, 'web_src/fomantic/build/semantic.js'),
      resolve(__dirname, 'web_src/js/index.js'),
      resolve(__dirname, 'web_src/fomantic/build/semantic.css'),
      resolve(__dirname, 'web_src/less/misc.css'),
      resolve(__dirname, 'web_src/less/index.less'),
    ],
    swagger: [
      resolve(__dirname, 'web_src/js/standalone/swagger.js'),
      resolve(__dirname, 'web_src/less/standalone/swagger.less'),
    ],
    serviceworker: [
      resolve(__dirname, 'web_src/js/serviceworker.js'),
    ],
    'eventsource.sharedworker': [
      resolve(__dirname, 'web_src/js/features/eventsource.sharedworker.js'),
    ],
    'easymde': [
      resolve(__dirname, 'web_src/js/easymde.js'),
      resolve(__dirname, 'node_modules/easymde/dist/easymde.min.css'),
    ],
    ...themes,
  },
  devtool: false,
  output: {
    path: resolve(__dirname, 'public'),
    filename: ({chunk}) => {
      // serviceworker can only manage assets below it's script's directory so
      // we have to put it in / instead of /js/
      return chunk.name === 'serviceworker' ? '[name].js' : 'js/[name].js';
    },
    chunkFilename: ({chunk}) => {
      const language = (/monaco.*languages?_.+?_(.+?)_/.exec(chunk.id) || [])[1];
      return language ? `js/monaco-language-${language.toLowerCase()}.js` : `js/[name].js`;
    },
  },
  optimization: {
    minimize: isProduction,
    minimizer: [
      new ESBuildMinifyPlugin({
        target: 'es2015',
        minify: true,
        css: true,
        legalComments: 'none',
      }),
    ],
    splitChunks: {
      chunks: 'async',
      name: (_, chunks) => chunks.map((item) => item.name).join('-'),
    },
    moduleIds: 'named',
    chunkIds: 'named',
  },
  module: {
    rules: [
      {
        test: /\.vue$/,
        exclude: /node_modules/,
        loader: 'vue-loader',
      },
      {
        test: /\.worker\.js$/,
        exclude: /monaco/,
        use: [
          {
            loader: 'worker-loader',
            options: {
              inline: 'no-fallback',
            },
          },
        ],
      },
      {
        test: /\.js$/,
        exclude: /node_modules/,
        use: [
          {
            loader: 'esbuild-loader',
            options: {
              target: 'es2015'
            },
          },
        ],
      },
      {
        test: /.css$/i,
        use: [
          {
            loader: MiniCssExtractPlugin.loader,
          },
          {
            loader: 'css-loader',
            options: {
              sourceMap: true,
              url: filterCssImport,
              import: filterCssImport,
            },
          },
        ],
      },
      {
        test: /.less$/i,
        use: [
          {
            loader: MiniCssExtractPlugin.loader,
          },
          {
            loader: 'css-loader',
            options: {
              sourceMap: true,
              importLoaders: 1,
              url: filterCssImport,
              import: filterCssImport,
            },
          },
          {
            loader: 'less-loader',
            options: {
              sourceMap: true,
            },
          },
        ],
      },
      {
        test: /\.svg$/,
        include: resolve(__dirname, 'public/img/svg'),
        type: 'asset/source',
      },
      {
        test: /\.(ttf|woff2?)$/,
        type: 'asset/resource',
        generator: {
          filename: 'fonts/[name][ext]',
          publicPath: '../', // required to remove css/ path segment
        }
      },
      {
        test: /\.png$/i,
        type: 'asset/resource',
        generator: {
          filename: 'img/webpack/[name][ext]',
          publicPath: '../', // required to remove css/ path segment
        }
      },
    ],
  },
  plugins: [
    new VueLoaderPlugin(),
    new MiniCssExtractPlugin({
      filename: 'css/[name].css',
      chunkFilename: 'css/[name].css',
    }),
    new SourceMapDevToolPlugin({
      filename: '[file].map',
      include: [
        'js/index.js',
        'css/index.css',
      ],
    }),
    new MonacoWebpackPlugin({
      filename: 'js/monaco-[name].worker.js',
    }),
    isProduction ? new LicenseCheckerWebpackPlugin({
      outputFilename: 'js/licenses.txt',
      outputWriter: ({dependencies}) => {
        const line = '-'.repeat(80);
        return dependencies.map((module) => {
          const {name, version, licenseName, licenseText} = module;
          const body = wrapAnsi(licenseText || '', 80);
          return `${line}\n${name}@${version} - ${licenseName}\n${line}\n${body}`;
        }).join('\n');
      },
      override: {
        'jquery.are-you-sure@*': {licenseName: 'MIT'},
      },
    }) : new AddAssetPlugin('js/licenses.txt', `Licenses are disabled during development`),
  ],
  performance: {
    hints: false,
    maxEntrypointSize: Infinity,
    maxAssetSize: Infinity,
  },
  resolve: {
    symlinks: false,
    alias: {
      vue$: 'vue/dist/vue.esm.js', // needed because vue's default export is the runtime only
    },
  },
  watchOptions: {
    ignored: [
      'node_modules/**',
    ],
  },
  stats: {
    assetsSort: 'name',
    assetsSpace: Infinity,
    cached: false,
    cachedModules: false,
    children: false,
    chunkModules: false,
    chunkOrigins: false,
    chunksSort: 'name',
    colors: true,
    entrypoints: false,
    excludeAssets: [
      /^js\/monaco-language-.+\.js$/,
      !isProduction && /^js\/licenses.txt$/,
    ].filter((item) => !!item),
    groupAssetsByChunk: false,
    groupAssetsByEmitStatus: false,
    groupAssetsByInfo: false,
    groupModulesByAttributes: false,
    modules: false,
    reasons: false,
    runtimeModules: false,
  },
};
