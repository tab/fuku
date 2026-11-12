export interface InstallMethod {
  title: string;
  code: string;
}

export function getInstallMethods(): InstallMethod[] {
  return [
    {
      title: "Homebrew",
      code: `brew trust --formula tab/apps/fuku
brew install tab/apps/fuku`,
    },
    {
      title: "Install Script",
      code: "curl -fsSL https://getfuku.sh/install.sh | sh",
    },
    {
      title: "Build from Source",
      code: `git clone https://github.com/tab/fuku.git
cd fuku
go build -o cmd/fuku cmd/main.go
sudo ln -sf $(pwd)/cmd/fuku /usr/local/bin/fuku`,
    },
  ];
}
