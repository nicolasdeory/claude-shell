class GptTerm < Formula
    desc "CLI tool for GPT interactions in the terminal"
    homepage "https://github.com/nicolasdeory/claude-shell"
    version "1.0.0"
    
    if OS.mac?
      url "https://github.com/nicolasdeory/claude-shell/releases/download/v1.0.0/gpt-term-darwin-amd64"
      sha256 "143144be990e8017721a1030aadb1915bb732aaf8510062c3978fc09a0dc578b"
    elsif OS.linux?
      url "https://github.com/nicolasdeory/claude-shell/releases/download/v1.0.0/gpt-term-linux-amd64"
      sha256 "bc8df7de075e87a193c6e8c629d1b063932ba4770ebe8b766f366ea3023399fa"
    end
    
    def install
      bin.install "gpt-term-#{OS.mac? ? "darwin" : "linux"}-amd64" => "gpt-term"
    end
  end