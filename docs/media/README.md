# Xelif demo assets

The PNGs and GIF are rendered from Xelif's actual `model.View()` output, driven
against a disposable workspace by `TestExportReadmeDemo`. Captions are added by
`scripts/render-demo.py`. This is a UI-renderer walkthrough, not an OS terminal
recording and not evidence of live mouse/terminal behavior.

The default README image is static. The GIF is behind a disclosure so readers
can choose whether to view motion.

## Reproduce

1. Choose a new, nonexistent absolute directory for `XELIF_DEMO_OUTPUT`.
2. Set `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, and `XDG_CACHE_HOME`
   to separate subdirectories inside that output directory. Do not use your
   normal Xelif settings. Storage outside the new output directory is rejected.
3. Run `go test -tags xelif_demo -run '^TestExportReadmeDemo$' -count=1 ./src/internal`.
4. Install Pillow in a disposable Python environment, then run:

```sh
python scripts/render-demo.py PATH/frames.json PATH/NerdFontMono-Regular.ttf docs/media
```

The illustrated font is CaskaydiaMono Nerd Font Mono, size 18. No font files are
bundled. File contents, directory names, and dates are sample data. No user
documents, credentials, or unrelated desktop windows are captured.

Generated outputs: `overview.png`, `commands.png`, and `walkthrough.gif`.

The exporter uses an explicit build tag because it is documentation-generation
tooling, not an application test or part of normal builds.
