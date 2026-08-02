const path = require("path");
const HtmlWebpackPlugin = require("html-webpack-plugin");
const Dotenv = require("dotenv-webpack");
const CopyPlugin = require("copy-webpack-plugin");

// O modo (development | production) vem da CLI: `webpack --mode production`
// O webpack chama esta função com (env, argv) quando exportamos uma função.
module.exports = (_env, argv) => {
  const isProduction = argv.mode === "production";

  return {
    entry: "./src/index.js",
    output: {
      path: path.resolve(__dirname, "dist"),
      filename: isProduction ? "bundle.[contenthash:8].js" : "bundle.js",
      clean: true,
    },
    devServer: {
      host: "0.0.0.0",
      historyApiFallback: true,
    },

    module: {
      rules: [
        {
          test: /\.(js|jsx)$/,
          exclude: /node_modules/,
          use: {
            loader: "babel-loader",
            options: {
              presets: ["@babel/preset-react"],
            },
          },
        },
        {
          test: /\.css$/,
          use: [
            "style-loader",
            "css-loader",
            {
              loader: "postcss-loader",
              options: {
                postcssOptions: {
                  plugins: ["tailwindcss", "autoprefixer"],
                },
              },
            },
          ],
        },
      ],
    },
    plugins: [
      new Dotenv({ systemvars: true }),
      new HtmlWebpackPlugin({
        template: "./public/index.html",
      }),
      new CopyPlugin({
        patterns: [
          {
            from: "./node_modules/firebase/firebase-messaging-sw.js",
            to: "firebase-messaging-sw.js",
          },
          {
            from: "./public/_redirects",
            to: "_redirects",
          },
        ],
      }),
    ],

    resolve: {
      fallback: {
        path: require.resolve("path-browserify"),
        os: require.resolve("os-browserify/browser"),
        crypto: require.resolve("crypto-browserify"),
        buffer: require.resolve("buffer"),
        vm: require.resolve("vm-browserify"),
        stream: require.resolve("stream-browserify"),
      },
    },

    // Em produção o webpack minifica automaticamente (TerserPlugin);
    // em development geramos source maps para facilitar o debug.
    devtool: isProduction ? "source-map" : "eval-cheap-module-source-map",
  };
};
