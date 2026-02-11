use zed_extension_api::{self as zed};

const GITHUB_REPO: &str = "shopwareLabs/shopware-lsp";
const BINARY_NAME: &str = "shopware-lsp";

struct ShopwareExtension {
    cached_binary_path: Option<String>,
}

impl ShopwareExtension {
    fn asset_suffix_for_platform(os: zed::Os, arch: zed::Architecture) -> Option<&'static str> {
        match (os, arch) {
            (zed::Os::Mac, zed::Architecture::X8664) => Some("darwin_amd64.zip"),
            (zed::Os::Mac, zed::Architecture::Aarch64) => Some("darwin_arm64.zip"),
            (zed::Os::Linux, zed::Architecture::X8664) => Some("linux_amd64.zip"),
            (zed::Os::Linux, zed::Architecture::Aarch64) => Some("linux_arm64.zip"),
            _ => None,
        }
    }

    fn server_path(
        &mut self,
        language_server_id: &zed::LanguageServerId,
        worktree: &zed::Worktree,
    ) -> zed::Result<String> {
        if let Some(ref path) = self.cached_binary_path {
            let p = std::path::Path::new(path);
            if p.exists() && p.is_file() {
                return Ok(path.clone());
            }
            self.cached_binary_path = None;
        }

        let root_path = worktree.root_path();
        let dev_paths = [
            format!("{}/shopware-lsp", root_path),
            format!("{}/../shopware-lsp/shopware-lsp", root_path),
        ];
        for path in &dev_paths {
            let p = std::path::Path::new(path);
            if p.exists() && p.is_file() {
                self.cached_binary_path = Some(path.clone());
                return Ok(path.clone());
            }
        }

        zed::set_language_server_installation_status(
            language_server_id,
            &zed::LanguageServerInstallationStatus::CheckingForUpdate,
        );

        let (os, arch) = zed::current_platform();
        let suffix = Self::asset_suffix_for_platform(os, arch)
            .ok_or_else(|| format!("Shopware LSP does not support {os:?} / {arch:?}"))?;

        let release = zed::latest_github_release(
            GITHUB_REPO,
            zed::GithubReleaseOptions {
                require_assets: true,
                pre_release: false,
            },
        )
        .map_err(|e| format!("Failed to fetch release: {e}"))?;

        let asset = release
            .assets
            .iter()
            .find(|a| a.name.ends_with(suffix))
            .ok_or_else(|| format!("No asset for {suffix} in release {}", release.version))?;

        zed::set_language_server_installation_status(
            language_server_id,
            &zed::LanguageServerInstallationStatus::Downloading,
        );

        zed::download_file(
            &asset.download_url,
            &asset.name,
            zed::DownloadedFileType::Zip,
        )
        .map_err(|e| format!("Failed to download: {e}"))?;

        let binary_path = BINARY_NAME.to_string();
        zed::make_file_executable(&binary_path)?;

        self.cached_binary_path = Some(binary_path.clone());
        Ok(binary_path)
    }
}

impl zed::Extension for ShopwareExtension {
    fn new() -> Self {
        Self {
            cached_binary_path: None,
        }
    }

    fn language_server_command(
        &mut self,
        language_server_id: &zed::LanguageServerId,
        worktree: &zed::Worktree,
    ) -> zed::Result<zed::Command> {
        let server_path = self.server_path(language_server_id, worktree)?;
        Ok(zed::Command {
            command: server_path,
            args: vec![],
            env: Default::default(),
        })
    }

    fn run_slash_command(
        &self,
        command: zed::SlashCommand,
        _args: Vec<String>,
        _worktree: Option<&zed::Worktree>,
    ) -> std::result::Result<zed::SlashCommandOutput, String> {
        match command.name.as_str() {
            "shopware-restart" => Ok(zed::SlashCommandOutput {
                text: "To restart the Shopware Language Server, reload Zed or disable and re-enable the Shopware LSP extension.".to_string(),
                sections: vec![],
            }),
            "shopware-reindex" => Ok(zed::SlashCommandOutput {
                text: "Zed does not yet support LSP workspace/executeCommand. To force reindex, reload the Shopware LSP extension or restart Zed. The server will reindex automatically when you open files.".to_string(),
                sections: vec![],
            }),
            _ => Err(format!("unknown slash command: {}", command.name)),
        }
    }
}

zed::register_extension!(ShopwareExtension);
