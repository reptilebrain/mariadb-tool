#!/usr/bin/env bash
# Render a badge to stdout; never modify the profile or tracked files.
set -euo pipefail

profile="${1:?Usage: bash .github/scripts/coverage-badge.sh coverage.out}"
coverage="$(go tool cover -func="$profile" | awk '$1 == "total:" { print $3 }')"
if [[ ! "$coverage" =~ ^[0-9]+([.][0-9]+)?%$ ]]; then
  printf 'Missing or invalid total statement coverage: %s\n' "$coverage" >&2
  exit 1
fi

color="#dfb317"
if awk "BEGIN { exit !(${coverage%\%} >= 80) }"; then
  color="#97ca00"
elif awk "BEGIN { exit !(${coverage%\%} < 60) }"; then
  color="#e05d44"
fi

cat <<SVG
<svg xmlns="http://www.w3.org/2000/svg" width="142" height="20" role="img" aria-label="unit coverage: $coverage">
  <title>Linux unit statement coverage: $coverage</title>
  <clipPath id="round"><rect width="142" height="20" rx="3"/></clipPath>
  <g clip-path="url(#round)">
    <rect width="94" height="20" fill="#555"/>
    <rect x="94" width="48" height="20" fill="$color"/>
  </g>
  <g fill="#fff" text-anchor="middle" font-family="Verdana,Geneva,DejaVu Sans,sans-serif" font-size="11">
    <text x="47" y="14">unit coverage</text>
    <text x="118" y="14">$coverage</text>
  </g>
</svg>
SVG
