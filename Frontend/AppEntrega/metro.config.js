// Learn more https://docs.expo.io/guides/customizing-metro
const { getDefaultConfig } = require("expo/metro-config");
const { withNativeWind } = require("nativewind/metro");

/** @type {import('expo/metro-config').MetroConfig} */
const config = getDefaultConfig(__dirname);

// NativeWind: processa o global.css e habilita a utility `className`.
module.exports = withNativeWind(config, { input: "./global.css" });
