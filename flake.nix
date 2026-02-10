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
        lib,
        makeWrapper,
      }:
        pkgs.buildGoModule {
          pname = "wayland-recorder";
          version = "0.1.0";
          src = ./.;
          vendorHash = "sha256-PTGksFXzBg5tdzpktqhdem/jNTHCKaxNhkvFrOi/0ag=";

          mainProgram = "wayland-recorder";

          nativeBuildInputs = [
            makeWrapper
          ];

          buildInputs = [
            pkgs.pipewire
            pkgs.gst_all_1.gstreamer
            pkgs.gst_all_1.gst-plugins-base
            pkgs.gst_all_1.gst-plugins-good
            pkgs.gst_all_1.gst-plugins-bad
            pkgs.gst_all_1.gst-plugins-ugly
          ];

          postInstall = ''
            wrapProgram $out/bin/wayland-recorder \
              --prefix PATH : ${lib.makeBinPath [
                pkgs.gst_all_1.gstreamer
              ]} \
              --prefix GST_PLUGIN_SYSTEM_PATH_1_0 : ${lib.makeSearchPath "lib/gstreamer-1.0" [
                pkgs.gst_all_1.gstreamer
                pkgs.gst_all_1.gst-plugins-base
                pkgs.gst_all_1.gst-plugins-good
                pkgs.gst_all_1.gst-plugins-bad
                pkgs.gst_all_1.gst-plugins-ugly
              ]}
          '';

          meta = defaultMeta;
        };
    };
}
