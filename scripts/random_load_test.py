import requests
import random
from concurrent.futures import ThreadPoolExecutor, as_completed

# Constants
VALID_URL = "http://localhost:8087/query?q=select * from xd where time > now() -  2w&db=mydb"
INVALID_URL = "http://localhost:8087/query?q=select * from xd where time > now() -  10w&db=mydb"
TOTAL_REQUESTS = 1000
VALID_PERCENT = 0.7

# Function to send a request
def send_request(url):
    try:
        response = requests.get(url, timeout=30)  # Grafana default timeout
        return url, response.status_code
    except requests.exceptions.RequestException as e:
        return url, str(e)

# Prepare mixed request list (70% valid, 30% invalid)
request_urls = [
    VALID_URL if random.random() < VALID_PERCENT else INVALID_URL
    for _ in range(TOTAL_REQUESTS)
]

# Run concurrent requests
def main():
    results = {
        "valid_200": 0,
        "invalid_403": 0,
        "other_status": {},
        "errors": 0
    }

    with ThreadPoolExecutor(max_workers=100) as executor:
        futures = [executor.submit(send_request, url) for url in request_urls]
        for future in as_completed(futures):
            url, status = future.result()
            if status == 200:
                results["valid_200"] += 1
            elif status == 403:
                results["invalid_403"] += 1
            elif isinstance(status, int):
                results["other_status"].setdefault(status, 0)
                results["other_status"][status] += 1
            else:
                results["errors"] += 1
                print(f"⚠️ Error for URL: {url} -> {status}")

    # Summary
    print("\n📊 Summary:")
    print(f"✅ Valid (200): {results['valid_200']}")
    print(f"❌ Invalid (403): {results['invalid_403']}")
    if results["other_status"]:
        print("⚠️ Other HTTP Status Codes:")
        for code, count in results["other_status"].items():
            print(f"  {code}: {count}")
    print(f"🔥 Exceptions/Errors: {results['errors']}")

if __name__ == "__main__":
    main()