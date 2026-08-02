const path = require("path");
const HtmlWebpackPlugin = require("html-webpack-plugin");
const CopyPlugin = require("copy-webpack-plugin");

// O modo (development | production) vem da CLI: `webpack --mode production`
// O webpack chama esta função com (env, argv) quando exportamos uma função.
module.exports = (_env, argv) => {
  const isProduction = argv.mode === "production";

  return {
    entry: "./src/index.js",
    output: {
      path: path.resolve(__dirname, "build"),
      filename: isProduction ? "bundle.[contenthash:8].js" : "bundle.js",
      publicPath: "/",
      clean: true,
    },
    devServer: {
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
              presets: ["@babel/preset-env", "@babel/preset-react"],
            },
          },
        },
        {
          test: /\.css$/,
          use: [
            "style-loader",
            { loader: "css-loader", options: { esModule: false } },
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
      new HtmlWebpackPlugin({
        template: "./public/index.html",
      }),
      new CopyPlugin({
        patterns: [
          { from: "public/_redirects", to: ".", noErrorOnMissing: true },
        ],
      }),
    ],
    resolve: {
      extensions: [".js", ".jsx"],
    },

    // Em produção o webpack minifica automaticamente (TerserPlugin);
    // em development geramos source maps para facilitar o debug.
    devtool: isProduction ? "source-map" : "eval-cheap-module-source-map",
  };
};
