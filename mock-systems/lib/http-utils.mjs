import http from "node:http";

export function readJson(req) {
  return new Promise((resolve, reject) => {
    let raw = "";
    req.on("data", (chunk) => {
      raw += chunk;
    });
    req.on("end", () => {
      if (!raw) {
        resolve({});
        return;
      }
      try {
        resolve(JSON.parse(raw));
      } catch (error) {
        reject(error);
      }
    });
  });
}

export function sendJson(res, statusCode, body) {
  res.writeHead(statusCode, {
    "content-type": "application/json; charset=utf-8",
    "access-control-allow-origin": "*"
  });
  res.end(JSON.stringify(body, null, 2));
}

export function createServer({ name, port, routes }) {
  const server = http.createServer(async (req, res) => {
    if (req.method === "OPTIONS") {
      sendJson(res, 200, { ok: true });
      return;
    }

    const url = new URL(req.url, `http://localhost:${port}`);
    const route = routes.find((candidate) => {
      return candidate.method === req.method && candidate.pattern.test(url.pathname);
    });

    if (!route) {
      sendJson(res, 404, { success: false, error: `${name}: route not found` });
      return;
    }

    try {
      const match = url.pathname.match(route.pattern);
      const body = await readJson(req);
      const result = await route.handler({ body, params: match?.groups ?? {}, query: url.searchParams });
      sendJson(res, result.statusCode ?? 200, result.body ?? result);
    } catch (error) {
      sendJson(res, 500, { success: false, error: error.message });
    }
  });

  server.listen(port, () => {
    console.log(`${name} listening on http://localhost:${port}`);
  });

  return server;
}
