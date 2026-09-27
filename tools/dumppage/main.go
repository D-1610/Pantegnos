// Command dumppage writes the rendered application and its stylesheet to a
// standalone HTML file, so the layout can be inspected and iterated on in a
// desktop browser at any viewport instead of only on a device:
//
//	go run ./tools/dumppage > page.html
package main

import (
	"fmt"

	"Pantegnos/internal/mobile"
)

func main() {
	fmt.Println(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">
<title>Pantegnos layout harness</title>
<style>`)
	fmt.Println(mobile.AppCSS())
	fmt.Println(`</style>
</head>
<body>`)
	fmt.Println(mobile.MarkupForTools())
	fmt.Println(`
<div id="probe" style="position:fixed;left:0;bottom:0;z-index:99;background:#000;color:#0f0;font:11px monospace;padding:2px 4px"></div>
<script>
// Reports what the stylesheet actually resolved to, which is the fastest way to
// tell "the media query is wrong" from "the rule is wrong".
(function () {
  var probe = document.getElementById("probe");
  var shell = document.querySelector(".shell");
  function report() {
    var cs = getComputedStyle(shell);
    var box = shell.getBoundingClientRect();
    probe.textContent =
      window.innerWidth + "x" + window.innerHeight +
      " dpr=" + window.devicePixelRatio +
      " display=" + cs.display +
      " cols=" + cs.gridTemplateColumns +
      " alignContent=" + cs.alignContent +
      " gap=" + cs.getPropertyValue("--gap").trim() +
      " w=" + Math.round(box.width);
  }
  report();
  window.addEventListener("resize", report);
})();
</script>
</body>
</html>`)
}
