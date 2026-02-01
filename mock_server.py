import http.server
import socketserver
import os
import sys

PORT = 8091
MOCK_DIR = "mock_data"

class MockHandler(http.server.SimpleHTTPRequestHandler):
    def do_GET(self):
        # APIのエンドポイントをローカルファイルにマッピング
        # url例: http://localhost:8080/api/shoplist -> mock_data/shoplist.xml
        
        # パスの最後の部分を取得 (shoplist, shopnewslist など)
        path = self.path.split('?')[0] # クエリパラメータ削除
        endpoint = path.rstrip('/').split('/')[-1]
        
        filename = f"{endpoint}.xml"
        file_path = os.path.join(MOCK_DIR, filename)
        
        if os.path.exists(file_path):
            self.send_response(200)
            self.send_header('Content-type', 'application/xml')
            self.end_headers()
            with open(file_path, 'rb') as f:
                self.wfile.write(f.read())
            print(f"Served: {file_path} for {self.path}")
        else:
            print(f"Not found: {file_path} for {self.path}")
            self.send_error(404, f"File not found: {filename}")

if __name__ == "__main__":
    if not os.path.exists(MOCK_DIR):
        os.makedirs(MOCK_DIR)
        print(f"Created directory: {MOCK_DIR}")

    print(f"Starting mock server on port {PORT}")
    print(f"Serving XML files from ./{MOCK_DIR}")
    print("Press Ctrl+C to stop")
    
    # Allow address reuse
    socketserver.TCPServer.allow_reuse_address = True
    
    try:
        with socketserver.TCPServer(("", PORT), MockHandler) as httpd:
            httpd.serve_forever()
    except KeyboardInterrupt:
        print("\nStopping server...")
        sys.exit(0)








