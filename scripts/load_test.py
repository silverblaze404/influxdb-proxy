import requests
import time
import random
from concurrent.futures import ThreadPoolExecutor, as_completed

# --- Configuration ---
# Your InfluxDB endpoint
INFLUXDB_URL = "http://localhost:8087"

# Database and measurement names
DATABASE = "mydb"
MEASUREMENT = "xd"

# Number of concurrent workers (how many requests to send at the same time)
# Adjust this based on your client machine's capabilities.
CONCURRENT_WORKERS = 50

# Query distribution
TOTAL_QUERIES = 1000
TIME_BOUND_QUERY_COUNT = 700
FULL_SCAN_QUERY_COUNT = 300

# --- Query Definitions ---
# Query 1: Selects data from the last 1 day
QUERY_TIME_BOUND = f"SELECT * FROM {DATABASE}..{MEASUREMENT} WHERE time > now() - 1d"

# Query 2: Selects all data (potentially very heavy)
QUERY_FULL_SCAN = f"SELECT * FROM {DATABASE}..{MEASUREMENT}"


def execute_query(query: str):
    """
    Executes a single InfluxDB query and returns its status and latency.
    
    A request is considered a FAILURE only on a 500 status code or a
    request-level exception (e.g., connection/read timeout). All other
    status codes are considered a SUCCESS for this load test.

    Args:
        query (str): The InfluxQL query to execute.

    Returns:
        tuple: A tuple containing (bool: success, float: latency in seconds).
               Returns (False, 0.0) on failure.
    """
    start_time = time.perf_counter()
    try:
        response = requests.get(
            f"{INFLUXDB_URL}/query",
            params={'q': query},
            timeout=300 # Set a generous timeout for potentially long queries
        )
        latency = time.perf_counter() - start_time
        
        # *** MODIFIED LOGIC: Only a 500 error is considered a failure. ***
        if response.status_code == 500:
            # Trim the response text to avoid flooding the console
            response_text = response.text[:150] if response.text else ""
            print(f"\n[Failure] Received status code 500: {response_text}...")
            return (False, 0.0)
        else:
            # Any other status code (200, 400, 401, etc.) is a "success"
            return (True, latency)
            
    except requests.exceptions.RequestException as e:
        # This handles connection timeouts, read timeouts, etc.
        print(f"\n[Failure] A request exception occurred: {e}")
        return (False, 0.0)

def main():
    """
    Main function to run the load test.
    """
    print("--- InfluxDB Load Test Starting ---")
    print(f"Total Requests: {TOTAL_QUERIES} ({TIME_BOUND_QUERY_COUNT} time-bound, {FULL_SCAN_QUERY_COUNT} full-scan)")
    print(f"Concurrency Level: {CONCURRENT_WORKERS} workers")
    print("Failure conditions: HTTP 500 or Connection/Read Timeouts")
    print("-" * 55)

    # Create a list of all queries to be executed
    queries_to_run = (
        [QUERY_TIME_BOUND] * TIME_BOUND_QUERY_COUNT +
        [QUERY_FULL_SCAN] * FULL_SCAN_QUERY_COUNT
    )
    
    # Shuffle the list to mix the query types for a more realistic load
    random.shuffle(queries_to_run)

    results = []
    start_time = time.perf_counter()

    # Use ThreadPoolExecutor for concurrent execution
    with ThreadPoolExecutor(max_workers=CONCURRENT_WORKERS) as executor:
        # Submit all tasks to the executor
        future_to_query = {executor.submit(execute_query, query): query for query in queries_to_run}
        
        # Process results as they complete
        for i, future in enumerate(as_completed(future_to_query)):
            results.append(future.result())
            # Simple progress indicator
            progress = (i + 1) / TOTAL_QUERIES * 100
            print(f"\rProgress: {i+1}/{TOTAL_QUERIES} ({progress:.2f}%)", end="")

    total_time = time.perf_counter() - start_time
    print("\n\n--- Load Test Finished ---")

    # --- Analysis & Reporting ---
    successes = [res for res in results if res[0]]
    failures = len(results) - len(successes)
    latencies = [res[1] for res in successes]

    print("\n--- Summary ---")
    print(f"Total time taken: {total_time:.2f} seconds")
    print(f"Total requests:   {TOTAL_QUERIES}")
    print(f"Successful:         {len(successes)}")
    print(f"Failed:             {failures}")
    
    if total_time > 0:
        rps = TOTAL_QUERIES / total_time
        print(f"Requests Per Second (RPS): {rps:.2f}")

    if latencies:
        avg_latency = sum(latencies) / len(latencies)
        min_latency = min(latencies)
        max_latency = max(latencies)
        print("\n--- Latency (for successful requests) ---")
        print(f"Average: {avg_latency:.4f} seconds")
        print(f"Min:     {min_latency:.4f} seconds")
        print(f"Max:     {max_latency:.4f} seconds")
    else:
        print("\nNo successful requests to analyze latency.")

if __name__ == "__main__":
    main()
