/**
 * Utilitários compartilhados entre scripts do plugin.
 */
function isMonorepoRoot(dir) {
  const fs = require("node:fs");
  const path = require("node:path");
  return (
    fs.existsSync(path.join(dir, "package.json")) ||
    fs.existsSync(path.join(dir, "go.work")) ||
    fs.existsSync(path.join(dir, "go.mod"))
  );
}

module.exports = { isMonorepoRoot };
