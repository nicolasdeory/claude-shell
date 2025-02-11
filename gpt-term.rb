class GptTerm < Formula
    desc "CLI tool for GPT interactions in the terminal"
    homepage "https://github.com/nicolasdeory/claude-shell"
    version "1.0.0"
    
    if OS.mac?
      url "https://github.com/nicolasdeory/claude-shell/releases/download/v1.0.0/gpt-term-darwin-amd64"
      sha256 "YOUR_DARWIN_BINARY_SHA256"
    elsif OS.linux?
      url "https://github.com/nicolasdeory/claude-shell/releases/download/v1.0.0/gpt-term-linux-amd64"
      sha256 "YOUR_LINUX_BINARY_SHA256"
    end
    
    def install
      bin.install "gpt-term-#{OS.mac? ? "darwin" : "linux"}-amd64" => "gpt-term"
    end
  end