#!/usr/bin/env node
// MCP server for the addons.mozilla.org (AMO) v5 add-on API.
// Lets an MCP client edit this add-on's listing metadata and upload new
// versions. Auth is AMO's JWT scheme: generate credentials at
// https://addons.mozilla.org/en-US/developers/addon/api/key/ and export
// AMO_JWT_ISSUER / AMO_JWT_SECRET. AMO_ADDON_ID sets the default add-on.
// API reference: https://mozilla.github.io/addons-server/topics/api/addons.html

import { createHmac, randomUUID } from "node:crypto";
import { readFile } from "node:fs/promises";
import { basename } from "node:path";

import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { z } from "zod";

const API_BASE = (process.env.AMO_API_BASE || "https://addons.mozilla.org/api/v5").replace(/\/$/, "");
const DEFAULT_ADDON = process.env.AMO_ADDON_ID || "";

// AMO wants a short-lived HS256 JWT per request: `Authorization: JWT <token>`.
function authHeader() {
  const issuer = process.env.AMO_JWT_ISSUER;
  const secret = process.env.AMO_JWT_SECRET;
  if (!issuer || !secret) {
    throw new Error(
      "AMO_JWT_ISSUER / AMO_JWT_SECRET are not set. Generate API credentials at " +
        "https://addons.mozilla.org/en-US/developers/addon/api/key/ and export them."
    );
  }
  const b64 = (obj) => Buffer.from(JSON.stringify(obj)).toString("base64url");
  const iat = Math.floor(Date.now() / 1000);
  const unsigned = `${b64({ alg: "HS256", typ: "JWT" })}.${b64({ iss: issuer, jti: randomUUID(), iat, exp: iat + 300 })}`;
  const sig = createHmac("sha256", secret).update(unsigned).digest("base64url");
  return `JWT ${unsigned}.${sig}`;
}

async function amo(method, path, { json, form } = {}) {
  const headers = { Authorization: authHeader() };
  let body;
  if (form) body = form;
  else if (json !== undefined) {
    headers["Content-Type"] = "application/json";
    body = JSON.stringify(json);
  }
  const res = await fetch(`${API_BASE}${path}`, { method, headers, body });
  const text = await res.text();
  if (!res.ok) throw new Error(`${method} ${path} -> HTTP ${res.status}\n${text}`);
  return text ? JSON.parse(text) : null;
}

const addonPath = (addon) => {
  const id = addon || DEFAULT_ADDON;
  if (!id) throw new Error("No add-on given and AMO_ADDON_ID is not set.");
  return `/addons/addon/${encodeURIComponent(id)}`;
};

// AMO localizes most text fields; accept plain strings and wrap as en-US.
const l10n = (v) => (typeof v === "string" ? { "en-US": v } : v);

const asResult = (data) => ({ content: [{ type: "text", text: JSON.stringify(data, null, 2) }] });

const server = new McpServer({ name: "amo", version: "0.1.0" });

const ADDON_ARG = z
  .string()
  .optional()
  .describe(`Add-on slug, GUID, or numeric id (default: ${DEFAULT_ADDON || "unset"})`);

server.registerTool(
  "get_addon",
  {
    description: "Fetch an add-on's full AMO detail (listing metadata, status, latest version).",
    inputSchema: { addon: ADDON_ARG },
  },
  async ({ addon }) => asResult(await amo("GET", `${addonPath(addon)}/`))
);

server.registerTool(
  "update_addon",
  {
    description:
      "PATCH an add-on's listing metadata. String fields are wrapped as en-US localized values. " +
      "Use `extra` to pass any other raw API fields (e.g. {\"categories\": [\"web-development\"]}).",
    inputSchema: {
      addon: ADDON_ARG,
      name: z.string().optional(),
      summary: z.string().max(250).optional().describe("Short summary shown in search (max 250 chars)"),
      description: z.string().optional().describe("Long listing description (limited HTML allowed)"),
      homepage: z.string().url().optional(),
      support_email: z.string().email().optional(),
      tags: z.array(z.string()).optional().describe("Slugs from AMO's fixed tag list"),
      is_experimental: z.boolean().optional(),
      extra: z.record(z.string(), z.unknown()).optional().describe("Raw fields merged into the PATCH body"),
    },
  },
  async ({ addon, name, summary, description, homepage, support_email, tags, is_experimental, extra }) => {
    const body = { ...extra };
    if (name !== undefined) body.name = l10n(name);
    if (summary !== undefined) body.summary = l10n(summary);
    if (description !== undefined) body.description = l10n(description);
    if (homepage !== undefined) body.homepage = l10n(homepage);
    if (support_email !== undefined) body.support_email = l10n(support_email);
    if (tags !== undefined) body.tags = tags;
    if (is_experimental !== undefined) body.is_experimental = is_experimental;
    if (!Object.keys(body).length) throw new Error("Nothing to update — pass at least one field.");
    return asResult(await amo("PATCH", `${addonPath(addon)}/`, { json: body }));
  }
);

server.registerTool(
  "upload_xpi",
  {
    description:
      "Upload a .zip/.xpi to AMO and wait for validation. Returns the upload uuid to pass to create_version.",
    inputSchema: {
      file: z.string().describe("Path to the built .zip/.xpi (e.g. web-ext-artifacts/*.zip)"),
      channel: z.enum(["listed", "unlisted"]).default("listed"),
    },
  },
  async ({ file, channel }) => {
    const form = new FormData();
    form.append("upload", new Blob([await readFile(file)], { type: "application/zip" }), basename(file));
    form.append("channel", channel);
    let upload = await amo("POST", "/addons/upload/", { form });
    for (let i = 0; i < 30 && !upload.processed; i++) {
      await new Promise((r) => setTimeout(r, 2000));
      upload = await amo("GET", `/addons/upload/${upload.uuid}/`);
    }
    const { uuid, processed, valid, validation } = upload;
    return asResult({
      uuid,
      processed,
      valid,
      errors: validation?.errors,
      warnings: validation?.warnings,
      messages: validation?.messages?.slice(0, 20),
    });
  }
);

server.registerTool(
  "create_version",
  {
    description: "Create a new version of an add-on from a validated upload uuid (submits for review).",
    inputSchema: {
      addon: ADDON_ARG,
      upload: z.string().describe("Upload uuid from upload_xpi (must have valid: true)"),
      release_notes: z.string().optional(),
      license: z.string().optional().describe("License slug, e.g. GPL-3.0-or-later — usually inherited"),
      compatibility: z.record(z.string(), z.unknown()).optional().describe('e.g. {"firefox": {"min": "115.0"}}'),
    },
  },
  async ({ addon, upload, release_notes, license, compatibility }) => {
    const body = { upload };
    if (release_notes !== undefined) body.release_notes = l10n(release_notes);
    if (license !== undefined) body.license = license;
    if (compatibility !== undefined) body.compatibility = compatibility;
    return asResult(await amo("POST", `${addonPath(addon)}/versions/`, { json: body }));
  }
);

server.registerTool(
  "update_version",
  {
    description: "PATCH an existing version (e.g. fix release notes).",
    inputSchema: {
      addon: ADDON_ARG,
      version: z.string().describe("Version id or version number"),
      release_notes: z.string().optional(),
      license: z.string().optional(),
      extra: z.record(z.string(), z.unknown()).optional().describe("Raw fields merged into the PATCH body"),
    },
  },
  async ({ addon, version, release_notes, license, extra }) => {
    const body = { ...extra };
    if (release_notes !== undefined) body.release_notes = l10n(release_notes);
    if (license !== undefined) body.license = license;
    if (!Object.keys(body).length) throw new Error("Nothing to update — pass at least one field.");
    return asResult(await amo("PATCH", `${addonPath(addon)}/versions/${encodeURIComponent(version)}/`, { json: body }));
  }
);

server.registerTool(
  "list_versions",
  {
    description: "List an add-on's versions with their review/approval status.",
    inputSchema: {
      addon: ADDON_ARG,
      filter: z
        .enum(["all_with_deleted", "all_with_unlisted", "all_without_unlisted"])
        .optional()
        .describe("Author-only filters; omit for public listed versions"),
    },
  },
  async ({ addon, filter }) => {
    const qs = filter ? `?filter=${filter}` : "";
    return asResult(await amo("GET", `${addonPath(addon)}/versions/${qs}`));
  }
);

await server.connect(new StdioServerTransport());
