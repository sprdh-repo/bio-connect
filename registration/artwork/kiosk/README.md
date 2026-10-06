# Self-service kiosk illustrations

Source PNGs for the six illustrations on `/kiosk`, generated with the Codex CLI image tool from `brief.md`.
The kiosk serves WebP copies from `internal/app/web/kiosk/`.

Regenerate the WebP copies after changing a source:

```sh
for f in scan print lanyard welcome helpdesk notfound; do
  magick artwork/kiosk/$f.png -resize 1200x800 -quality 84 -define webp:method=6 internal/app/web/kiosk/$f.webp
done
```

The backgrounds are the page cream (`#F5F3EB`) within a level or two and the kiosk places them on cream surfaces, so they read as cut-outs; keep any new artwork on that exact background.
