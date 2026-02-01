{
  inputs.flakelight.url = "github:nix-community/flakelight";
  outputs = {flakelight, ...}:
    flakelight ./. {
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
    };
}
