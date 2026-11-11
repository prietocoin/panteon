package davinci

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"

	"elpanteon/engine/internal/galileo/contract"
)

// Page es el HTML procedural de un clip atómico.
type Page struct {
	ClipID string
	Path   string
	HTML   string
}

// RenderClip genera el DOM procedural para un clip.
// No usa plantillas rígidas: compone primitivas según Visual.Kind.
func RenderClip(clip contract.Clip, outDir string, width, height int) (Page, error) {
	if strings.TrimSpace(clip.ID) == "" {
		return Page{}, fmt.Errorf("davinci: clip sin id")
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return Page{}, fmt.Errorf("davinci: creando %s: %w", outDir, err)
	}

	document := buildDocument(clip, width, height)
	path := filepath.Join(outDir, clip.ID+".html")
	if err := os.WriteFile(path, []byte(document), 0o644); err != nil {
		return Page{}, fmt.Errorf("davinci: escribiendo %s: %w", path, err)
	}
	return Page{ClipID: clip.ID, Path: path, HTML: document}, nil
}

func buildDocument(clip contract.Clip, width, height int) string {
	var builder strings.Builder
	builder.WriteString("<!DOCTYPE html>\n<html lang=\"es\">\n<head>\n")
	builder.WriteString("<meta charset=\"UTF-8\">\n")
	builder.WriteString(fmt.Sprintf("<meta name=\"clip-id\" content=\"%s\">\n", html.EscapeString(clip.ID)))
	builder.WriteString(fmt.Sprintf("<meta name=\"canvas\" content=\"%dx%d\">\n", width, height))
	builder.WriteString(fmt.Sprintf("<meta name=\"duration\" content=\"%.3f\">\n", clip.DurationSec))
	builder.WriteString("<title>")
	builder.WriteString(html.EscapeString(clip.Visual.Title))
	builder.WriteString("</title>\n<style>")
	builder.WriteString(css())
	builder.WriteString("</style>\n</head>\n<body>\n")
	builder.WriteString("<main class=\"canvas\">\n")
	builder.WriteString("<div class=\"accent\"></div>\n")

	if len(clip.Visual.Beats) > 0 {
		builder.WriteString(renderBeats(clip.Visual.Beats))
	} else {
		switch clip.Visual.Kind {
		case "title":
			builder.WriteString(renderTitle(clip.Visual))
		case "cards":
			builder.WriteString(renderCards(clip.Visual))
		case "bullets":
			builder.WriteString(renderBullets(clip.Visual))
		case "code":
			builder.WriteString(renderCode(clip.Visual))
		default:
			builder.WriteString(renderTitle(clip.Visual))
		}
	}

	builder.WriteString("<footer><span>El Panteón · Galileo</span><span class=\"counter\">")
	builder.WriteString(html.EscapeString(clip.ID))
	builder.WriteString("</span></footer>\n<div class=\"progress\"></div>\n</main>\n")
	builder.WriteString(script())
	builder.WriteString("\n</body>\n</html>\n")
	return builder.String()
}

func renderBeats(beats []contract.Beat) string {
	var builder strings.Builder
	for _, beat := range beats {
		builder.WriteString(fmt.Sprintf(
			"<div class=\"beat\" data-start=\"%.4f\" data-end=\"%.4f\">\n",
			beat.Start, beat.End,
		))
		visual := contract.Visual{
			Kind:         beat.Kind,
			Title:        beat.Title,
			Subtitle:     beat.Subtitle,
			Bullets:      beat.Bullets,
			Cards:        beat.Cards,
			Code:         beat.Code,
			CodeLanguage: beat.CodeLanguage,
		}
		switch beat.Kind {
		case "cards":
			builder.WriteString(renderCards(visual))
		case "bullets":
			builder.WriteString(renderBullets(visual))
		case "code":
			builder.WriteString(renderCode(visual))
		default:
			builder.WriteString(renderTitle(visual))
		}
		builder.WriteString("</div>\n")
	}
	return builder.String()
}

func renderTitle(visual contract.Visual) string {
	var builder strings.Builder
	builder.WriteString("<section class=\"stage center\">\n")
	builder.WriteString("<div class=\"eyebrow reveal\" style=\"--i:0\">Caso de ingeniería</div>\n")
	builder.WriteString("<h1 class=\"reveal\" style=\"--i:1\">")
	builder.WriteString(html.EscapeString(visual.Title))
	builder.WriteString("</h1>\n")
	if visual.Subtitle != "" {
		builder.WriteString("<p class=\"subtitle reveal\" style=\"--i:2\">")
		builder.WriteString(html.EscapeString(visual.Subtitle))
		builder.WriteString("</p>\n")
	}
	builder.WriteString("</section>\n")
	return builder.String()
}

func renderCards(visual contract.Visual) string {
	var builder strings.Builder
	builder.WriteString("<section class=\"stage\">\n")
	builder.WriteString("<header class=\"reveal\" style=\"--i:0\"><span class=\"kicker\">Evidencia</span><h2>")
	builder.WriteString(html.EscapeString(visual.Title))
	builder.WriteString("</h2></header>\n<div class=\"grid\">\n")
	for index, card := range visual.Cards {
		builder.WriteString(fmt.Sprintf("<article class=\"card tone-%s reveal\" style=\"--i:%d\">\n", tone(card.Tone), index+1))
		builder.WriteString("<h3>")
		builder.WriteString(html.EscapeString(card.Title))
		builder.WriteString("</h3><p>")
		builder.WriteString(html.EscapeString(card.Body))
		builder.WriteString("</p></article>\n")
	}
	builder.WriteString("</div>\n</section>\n")
	return builder.String()
}

func renderBullets(visual contract.Visual) string {
	var builder strings.Builder
	builder.WriteString("<section class=\"stage\">\n")
	builder.WriteString("<header class=\"reveal\" style=\"--i:0\"><span class=\"kicker\">Análisis</span><h2>")
	builder.WriteString(html.EscapeString(visual.Title))
	builder.WriteString("</h2></header>\n<ul class=\"bullets\">\n")
	for index, bullet := range visual.Bullets {
		builder.WriteString(fmt.Sprintf("<li class=\"reveal\" style=\"--i:%d\"><span class=\"dot\"></span><p>", index+1))
		builder.WriteString(html.EscapeString(bullet))
		builder.WriteString("</p></li>\n")
	}
	builder.WriteString("</ul>\n</section>\n")
	return builder.String()
}

func renderCode(visual contract.Visual) string {
	var builder strings.Builder
	builder.WriteString("<section class=\"stage\">\n")
	builder.WriteString("<header class=\"reveal\" style=\"--i:0\"><span class=\"kicker\">")
	builder.WriteString(html.EscapeString(strings.ToUpper(visual.CodeLanguage)))
	builder.WriteString("</span><h2>")
	builder.WriteString(html.EscapeString(visual.Title))
	builder.WriteString("</h2></header>\n<pre class=\"code reveal\" style=\"--i:1\">")
	for _, line := range strings.Split(visual.Code, "\n") {
		class := "line"
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "#"):
			class += " comment"
		case strings.HasPrefix(trimmed, "-"):
			class += " action"
		case strings.Contains(line, ":"):
			class += " key"
		}
		builder.WriteString(fmt.Sprintf("<span class=\"%s\">%s</span>\n", class, html.EscapeString(line)))
	}
	builder.WriteString("</pre>\n</section>\n")
	return builder.String()
}

func tone(value string) string {
	switch value {
	case "danger", "success", "warning", "neutral":
		return value
	default:
		return "neutral"
	}
}

func css() string {
	return fmt.Sprintf(`
:root { --p: 0; --bg: #0f172a; --panel: #1e293b; --line: #334155;
  --text: #f8fafc; --muted: #94a3b8; --sky: #38bdf8; --green: #10b981;
  --red: #f43f5e; --amber: #f59e0b; }
* { box-sizing: border-box; margin: 0; padding: 0; }
html, body { width: 100%%; height: 100%%; overflow: hidden; background: var(--bg); }
body { font-family: "Segoe UI", Roboto, Helvetica, Arial, sans-serif; color: var(--text); }
.canvas { position: relative; width: 100vw; height: 100vh; padding: 5vh 6vw 8vh;
  background: radial-gradient(circle at 80%% 10%%, #1e3a5f 0%%, transparent 45%%),
              radial-gradient(circle at 10%% 90%%, #052e2b 0%%, transparent 40%%), var(--bg); }
.accent { position: absolute; top: 0; left: 0; height: 4px; width: 100%%;
  background: linear-gradient(90deg, var(--sky), var(--green)); }
.stage { height: 100%%; display: flex; flex-direction: column; gap: 3vh; }
.stage.center { align-items: center; justify-content: center; text-align: center; }
.eyebrow, .kicker { color: var(--sky); text-transform: uppercase; letter-spacing: .22em;
  font-size: clamp(10px, 1.2vw, 16px); font-weight: 700; }
h1 { font-size: clamp(28px, 6vw, 92px); line-height: 1.02; max-width: 90%%; }
h2 { font-size: clamp(24px, 4vw, 60px); line-height: 1.05; margin-top: 1vh; }
.subtitle { color: var(--muted); font-size: clamp(14px, 1.8vw, 26px); max-width: 75%%; }
header { border-bottom: 1px solid var(--line); padding-bottom: 2vh; }
.grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 2vw;
  align-content: center; flex: 1; }
.card { background: var(--panel); border: 1px solid var(--line); border-left: 6px solid var(--sky);
  padding: 3vh 2vw; display: flex; flex-direction: column; gap: 1.4vh; }
.card h3 { font-size: clamp(16px, 2vw, 30px); }
.card p { color: #cbd5e1; font-size: clamp(12px, 1.4vw, 21px); line-height: 1.45; }
.tone-danger { border-left-color: var(--red); }
.tone-success { border-left-color: var(--green); }
.tone-warning { border-left-color: var(--amber); }
.bullets { list-style: none; display: flex; flex-direction: column; gap: 2.2vh;
  justify-content: center; flex: 1; }
.bullets li { display: flex; gap: 1.4vw; align-items: flex-start;
  background: rgba(30, 41, 59, .6); border: 1px solid var(--line); padding: 2vh 1.6vw; }
.bullets p { font-size: clamp(14px, 1.7vw, 25px); line-height: 1.35; }
.dot { width: 10px; height: 10px; margin-top: .7vh; flex: 0 0 10px; background: var(--green); }
.code { flex: 1; overflow: hidden; background: #020617; border: 1px solid var(--line);
  padding: 3vh 2vw; font-family: Consolas, "Courier New", monospace;
  font-size: clamp(11px, 1.25vw, 19px); line-height: 1.6; white-space: pre-wrap; }
.line { display: block; }
.comment { color: #64748b; }
.action { color: var(--green); }
.key { color: var(--sky); }
footer { position: absolute; left: 6vw; right: 6vw; bottom: 3.4vh; display: flex;
  justify-content: space-between; color: #64748b; font-size: clamp(9px, 1vw, 14px);
  letter-spacing: .14em; text-transform: uppercase; }
.progress { position: absolute; left: 0; bottom: 0; height: 5px; width: 100%%;
  background: rgba(51, 65, 85, .5); }
.progress::after { content: ""; display: block; height: 100%%; width: calc(var(--p) * 100%%);
  background: linear-gradient(90deg, var(--sky), var(--green)); }
.reveal { opacity: calc(var(--p) * 1.5 - var(--i) * .12);
  transform: translateY(calc((1 - var(--p)) * 26px)); }
.beat { display: none; height: 100%%; }
.beat.active { display: block; }
`)
}

func script() string {
	return `<script>
(function () {
  var params = new URLSearchParams(window.location.search);
  var frame = Number(params.get('frame') || 0);
  var total = Math.max(1, Number(params.get('total') || 1));
  var meta = document.querySelector('meta[name="duration"]');
  var duration = Number(params.get('duration') || (meta ? meta.content : 0)) || 1;
  window.setFrame = function (index, count, seconds) {
    var p = count <= 1 ? 1 : index / (count - 1);
    p = Math.max(0, Math.min(1, p));
    document.documentElement.style.setProperty('--p', String(p));
    var d = Number(seconds) || duration;
    var t = p * d;
    document.querySelectorAll('.beat').forEach(function (beat) {
      var start = Number(beat.dataset.start || 0) * d;
      var end = Number(beat.dataset.end || 1) * d;
      beat.classList.toggle('active', t >= start && (t < end || (p >= 1 && t <= end)));
    });
  };
  window.advanceAnimation = window.setFrame;
  window.setFrame(frame, total, duration);
})();
</script>`
}
