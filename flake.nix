{
  inputs.flakelight.url = "github:nix-community/flakelight";
  outputs = {flakelight, ...}:
    flakelight ./. {
      pname = "wayland-recorder";

      devShell.packages = pkgs: [
        pkgs.go
        pkgs.gopls
        pkgs.cobra-cli
        pkgs.pipewire
        pkgs.gst_all_1.gstreamer
        pkgs.gst_all_1.gst-plugins-base
        pkgs.gst_all_1.gst-plugins-good
        pkgs.gst_all_1.gst-plugins-bad
        pkgs.gst_all_1.gst-plugins-ugly

        pkgs.just
      ];
      package = {
        stdenv,
        defaultMeta,
        pkgs,
      }:
        pkgs.buildGoModule {
          pname = "wayland-recorder";
          version = "0.1.0";
          src = ./.;
          vendorHash = "sha256-PTGksFXzBg5tdzpktqhdem/jNTHCKaxNhkvFrOi/0ag=";

          mainProgram = "wayland-recorder";

          buildInputs = [
            pkgs.pipewire
            pkgs.gst_all_1.gstreamer
            pkgs.gst_all_1.gst-plugins-base
            pkgs.gst_all_1.gst-plugins-good
            pkgs.gst_all_1.gst-plugins-bad
            pkgs.gst_all_1.gst-plugins-ugly
          ];

          meta = defaultMeta;
        };
    };
}
