build:
  go build -o output/wayland-recorder ./

run +args:
  go run ./ {{args}}