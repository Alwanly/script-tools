# rewrite_to_local
A mitmproxy addon that transparently reroutes all requests for your staging API host to your local service, driven by environment variables.

## 📁 Project Structure
rewrite_to_local/
├── .env.example # example env vars
├── requirements.txt # Python dependencies
├── rewrite_to_local.py # mitmproxy addon script
└── README.md # this file


## 🚀 Prerequisites

- Python 3.8+
- `pip` (or `pipx`)
- mitmproxy (v7+)
- A browser where you can install a custom proxy CA

## ⚙️ Installation

1. Clone this repo:

   ```bash
   git clone https://your.git.repo/rewrite_to_local.git
   cd rewrite_to_local

2. Install dependencies:
```
pip install -r requirements.txt
```

3. Copy and edit your .env:
`cp .env.example .env`
