# examples

Sample files for trying out pollen's import / configuration features.

| File | What to do with it |
|------|--------------------|
| `sample.openapi.yaml` | OpenAPI 3.0 spec with five endpoints. Inside pollen, press `Ctrl+I` and enter the path — every operation becomes a collection entry |
| `sample.postman.json` | Postman Collection v2.1 with three requests covering raw JSON body, request chaining (`{{response.body.token}}`), and url-encoded form body. Import with `Ctrl+I` |
| `settings.example.json` | Reference `settings.json` populated with every field at its default. Copy to `~/.config/pollen/settings.json` and tweak |
| `env.example.json` | Reference `env.json` with three environments (dev/staging/prod) and a couple of variables. Copy to `~/.config/pollen/env.json` and switch between them at runtime with `Ctrl+E` |

The endpoints in the samples point at `api.example.com` / `localhost:8080`
which won't respond — they are deliberately fictional so importing them is
safe. Edit the URLs to your own server, or change the active environment to
adjust them all at once via the `{{baseUrl}}` token.
