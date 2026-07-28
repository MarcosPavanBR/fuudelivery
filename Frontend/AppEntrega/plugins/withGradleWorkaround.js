// Plugin Expo para criar o software component 'release' que o
// expo-modules-core e react-native precisam para publicar Maven.
// Sem este plugin, o build Gradle falha com:
// "Could not get unknown property 'release' for SoftwareComponent container"
const { withAppBuildGradle } = require("expo/config-plugins");

const GRADLE_WORKAROUND = `
// WORKAROUND: Create 'release' software component for expo-modules-core
// and react-native publishing. Without this, Gradle fails with:
// "Could not get unknown property 'release' for SoftwareComponent container"
project.afterEvaluate {
    def releaseSoftwareComponent = project.components.findByName("release")
    if (releaseSoftwareComponent == null) {
        project.components.add(
            project.objects.newInstance(
                org.gradle.api.internal.component.DefaultSoftwareComponent,
                "release"
            )
        )
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
