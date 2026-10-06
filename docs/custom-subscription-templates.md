# Custom subscription page templates

We1BBoard can render subscription pages from your own HTML templates (same idea as 3x-ui).

## Setup

1. Create a directory on the server, e.g. `/etc/we1bboard/sub_templates/my-theme/`
2. Put `sub.html` (preferred) or `index.html` inside it
3. In panel **Settings → Subscription**, set **subThemeDir** to that absolute path
4. Save. Leave empty to use the built-in page

Malformed / missing templates fall back to the built-in page (subscription responses stay valid).

Theme files must be regular files (not symlinks). Keep the directory owned by the panel user and not world-writable (e.g. `0750`).

## Template engine

Go `html/template` (auto-escaping). Variables match 3x-ui naming:

| Variable | Type | Description |
|----------|------|-------------|
| `.sId` | string | Subscription id |
| `.enabled` | bool | Has active clients |
| `.isOnline` | bool | Reserved (currently always false) |
| `.download` / `.upload` / `.total` / `.used` / `.remained` | string | Formatted traffic |
| `.downloadByte` / `.uploadByte` / `.totalByte` | int64 | Bytes |
| `.expire` | int64 | Unix seconds (`0` = never) |
| `.lastOnline` | int64 | Unix ms (`0` = never) |
| `.subUrl` / `.subJsonUrl` / `.subClashUrl` / `.subSingboxUrl` | string | URLs |
| `.subTitle` / `.subSupportUrl` / `.announce` | string | Branding |
| `.links` | []string | Share URIs (**only** when opened with `?html=1`) |
| `.emails` | []string | Client emails |
| `.qrDataUrl` | string | Data-URL PNG of subscription URL |
| `.datepicker` | string | `"gregorian"` |

Browser navigation without `?html=1` is **copy-only**: `.links` is empty even in custom templates.

## Live status

```http
GET /sub/<subId>?format=info
```

Same view-model as JSON, without `links` — for polling from the template:

```html
<script>
async function refresh() {
  const r = await fetch(location.pathname + '?format=info');
  if (!r.ok) return;
  const info = await r.json();
  document.getElementById('used').textContent = info.used;
}
refresh();
setInterval(refresh, 10000);
</script>
```

## Example

See `scripts/sub_templates/example/sub.html`.
