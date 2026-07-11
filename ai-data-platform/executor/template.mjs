const tokenPattern = /\{\{\s*([^}]+?)\s*\}\}/g;

export function getPath(source, expression) {
  if (!expression) return undefined;
  if (expression.startsWith("$.")) {
    expression = expression.slice(2);
  }
  const parts = expression.split(".").filter(Boolean);
  let current = source;
  for (const part of parts) {
    if (current == null) return undefined;
    current = current[part];
  }
  return current;
}

function resolveExpression(expression, scope) {
  const options = expression.split("|").map((item) => item.trim());
  for (const option of options) {
    if (!option) continue;
    const value = option.includes(".") ? getPath(scope, option) : scope[option];
    if (value !== undefined && value !== null && value !== "") {
      return value;
    }
    if (!option.includes(".") && !["true", "false"].includes(option)) {
      return option;
    }
  }
  return "";
}

export function renderValue(value, scope) {
  if (typeof value === "string") {
    const exact = value.match(/^\{\{\s*([^}]+?)\s*\}\}$/);
    if (exact) {
      return resolveExpression(exact[1], scope);
    }
    return value.replace(tokenPattern, (_, expression) => String(resolveExpression(expression, scope)));
  }
  if (Array.isArray(value)) {
    return value.map((item) => renderValue(item, scope));
  }
  if (value && typeof value === "object") {
    const rendered = {};
    for (const [key, child] of Object.entries(value)) {
      rendered[key] = renderValue(child, scope);
    }
    return rendered;
  }
  return value;
}

export function isTruthyTemplate(value, scope) {
  const rendered = renderValue(value, scope);
  return rendered === true || rendered === "true" || Boolean(rendered);
}
