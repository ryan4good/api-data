import path from "node:path";
import { platformDir, readJson, readJsonDir } from "../lib/fs.mjs";

export async function loadScenario(scene) {
  const scenarios = await readJsonDir(path.join(platformDir, "scenarios"));
  const scenario = scenarios.find((item) => item.scene === scene || item.aliases?.includes(scene));
  if (!scenario) {
    throw new Error(`Scenario not found: ${scene}`);
  }
  return scenario;
}

export async function loadConnectors(env = "test") {
  const connectors = await readJsonDir(path.join(platformDir, "connectors"));
  return new Map(
    connectors
      .filter((connector) => connector.env === env)
      .map((connector) => [connector.system, connector])
  );
}

export async function loadSystems() {
  const systems = await readJsonDir(path.join(platformDir, "system-registry"));
  return new Map(systems.map((system) => [system.code, system]));
}
