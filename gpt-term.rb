class GptTerm < Formula
    desc "CLI tool for GPT interactions in the terminal"
    homepage "https://github.com/nicolasdeory/claude-shell"
    version "1.0.1"
    
    if OS.mac?
      url "https://github.com/nicolasdeory/claude-shell/releases/download/v1.0.1/gpt-term-darwin-amd64"
      sha256 "0a76f73f6bdd1a5fbdaa1735693403185b712569dfcff485150b964d0b991ace"
    elsif OS.linux?
      url "https://github.com/nicolasdeory/claude-shell/releases/download/v1.0.1/gpt-term-linux-amd64"
      sha256 "0f006bc1301ff4d8a391d4948b1eeb6512107c4ebfe056d33794070767cf35f6"
    end
    
    def install
      bin.install "gpt-term-#{OS.mac? ? "darwin" : "linux"}-amd64" => "gpt-term"
    end
  end
