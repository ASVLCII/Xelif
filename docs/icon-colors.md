# File-type icon colors

Decision: retain the recognizable original Nerd Font glyphs and apply a small categorical palette only to file-entry icons. Filenames, borders, backgrounds, and navigation stay neutral. This supersedes the earlier fully monochrome icon treatment.

## Evidence and limits

Sources consulted on 2026-10-21:

- [W3C: Use of Color](https://www.w3.org/WAI/WCAG22/Understanding/use-of-color.html) requires additional shape/text cues rather than color alone. Icons and filename extensions remain available; color is redundant assistance.
- [W3C: Non-text Contrast](https://www.w3.org/WAI/WCAG22/Understanding/non-text-contrast.html) gives a 3:1 minimum for meaningful graphics. Xelif targets 4.5:1 for these small glyphs as a conservative design choice, not a claim of WCAG certification. Theme RGB contrast is checked, not font anti-aliasing, display calibration, or every terminal's color conversion.
- [IBM Carbon: Color palettes](https://www.carbondesignsystem.com/building-blocks/data-visualization/color-palettes) describes categorical palettes for unrelated categories. Xelif uses a subset of Carbon hues, with darker variants for light backgrounds. It is not Carbon's prescribed chart sequence.
- [Tseng et al., CHI 2023: Measuring Categorical Perception in Color-Coded Scatterplots](https://arxiv.org/abs/2303.15583), submitted 2023-03-27, finds that palette discriminability and category count affect performance. This supports limiting categories as an inference, not a file-manager usability result: the study tested scatterplots, not terminal icons.

There is no universal scientific assignment of a particular hue to a file type. These category associations are product decisions, not psychological facts or standardized file meanings. They must be learned, and no claim of complete color-vision-deficiency separability is made. Icon shape and filename remain primary.

## Palette

| Category | Dark-background RGB | Light-background RGB | Design rationale |
| --- | --- | --- | --- |
| Folders and links | `#78a9ff` | `#0043ce` | Cool navigation cue; folder/link shapes distinguish them. |
| Images/design files | `#be95ff` | `#6929c4` | Distinct from the navigation and document groups; not a claim that purple inherently means imagery. |
| Documents, Markdown, tables | `#6fdc8c` | `#0e6027` | One consistent content family, including the Markdown M glyph. |
| Archives and disk images | `#f1c21b` | `#684e00` | Warm, distinct container category, not a warning. |
| Code/config/known technical formats | `#3ddbd9` | `#005d5d` | A unified technical family instead of arbitrary per-language brand colors. |
| Audio/video | `#ff7eb6` | `#9f1853` | One media family to keep the number of hues small. |
| Other/unknown | `#c6c6c6` | `#525252` | Neutral rather than assigning a misleading category. |

The resolver uses existing extension/full-name aliases, case-insensitively. Folders and links take precedence. Light/dark variants are chosen by measured background contrast; if neither reaches 4.5:1, the existing theme foreground is used. Selected rows are checked against their own background. Theme foreground fallback quality remains the theme author's responsibility. Unknown/transparent backgrounds also fall back because actual terminal contrast cannot be guaranteed.

Configuration: `color_file_icons = true` by default; set it to `false` to opt out. Plain mode suppresses category colors. The presence of `NO_COLOR`, including an empty value, suppresses icon foreground coloring.
