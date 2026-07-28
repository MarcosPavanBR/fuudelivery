// Plugin Expo para adicionar workaround do erro Gradle 'release' property
const { withAppBuildGradle } = require("expo/config-plugins");

const withGradleWorkaround = (config) => {
  return withAppBuildGradle(config, (config) => {
    const workaround = `
// WORKAROUND: Fix react-native-maps Gradle error
// "Could not get unknown property 'release' for SoftwareComponent container"
afterEvaluate {
    try {
        publishing {
            publications {
                release(MavenPublication) {
                    from components.release
                }
            }
        }
    } catch (Exception e) {
        // Ignore - not all projects have publishing configured
    }
}`;

    if (!config.modResults.contents.includes("WORKAROUND: Fix react-native-maps")) {
      config.modResults.contents += workaround;
    }
    return config;
  });
};

module.exports = withGradleWorkaround;
