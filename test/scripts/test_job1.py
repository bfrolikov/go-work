import os
import urllib.request
urllib.request.urlopen(f"http://localhost:{os.getenv('TEST_PING_SERVER_PORT')}/tests/ping/1/").read()
