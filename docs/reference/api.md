# REST API

All endpoints are JSON over HTTP, rooted at `/api/`. Errors return a JSON body
of the form `{"error": "message"}`.

## Health

### `GET /api/health`

```json
{ "status": "ok", "version": "v0.1.0" }
```

## Items

### `GET /api/items`

Returns all items, newest first.

```json
[
  { "id": 2, "title": "Second", "created_at": "2026-01-01T00:00:10Z" },
  { "id": 1, "title": "First", "created_at": "2026-01-01T00:00:00Z" }
]
```

### `POST /api/items`

Creates an item. The title is required and trimmed.

Request:

```json
{ "title": "First" }
```

Response: `201 Created` with the created item.

### `DELETE /api/items/{id}`

Deletes an item. Responds `204 No Content` on success, `404 Not Found` if the
item does not exist.
