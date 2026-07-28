// Plugin Expo para criar o software component 'release' que o
// expo-modules-core e react-native precisam para publicar Maven.
const { withAppBuildGradle } = require("expo/config-plugins");

const GRADLE_WORKAROUND = `
// WORKAROUND: Create 'release' software component for expo-modules-core
project.afterEvaluate {
    try {
        def pc = project.extensions.getByType(PublishingExtension)
        if (pc.publications.findByName("release") == null) {
            pc.publications.create("release", MavenPublication) {
                artifact = ""
            }
        }
    } catch (Exception e) {
        // Ignore
    }
}`;

const withGradleWorkaround = (config) => {
  return withAppBuildGradle(config, (config) => {
    if (!config.modResults.contents.includes("WORKAROUND: Create")) {
      config.modResults.contents += GRADLE_WORKAROUND;
    }
    return config;
  });
};

module.exports = withGradleWorkaround;
