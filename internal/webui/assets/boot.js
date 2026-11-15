"use strict";
// Apply saved appearance before the first paint, so the page never flashes the wrong theme.
(function () {
  try {
    var s = JSON.parse(localStorage.getItem("truss-settings") || "{}");
    var d = document.documentElement;
    if (s.theme === "light" || s.theme === "dark") d.dataset.theme = s.theme;
    if (s.density === "compact") d.dataset.density = "compact";
    if (s.sidebar === "collapsed") d.dataset.sidebar = "collapsed";
    if (s.motion === "reduced") d.dataset.motion = "reduced";
  } catch (e) { /* storage unavailable: defaults apply */ }
})();
