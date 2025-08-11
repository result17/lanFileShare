# Testing Service Discovery Delay

This document outlines the steps to manually test and observe the delay in service discovery when a `receiver` service is shut down.

## Objective

To measure the time it takes for the `sender` to recognize that a `receiver` service has gone offline.

## Prerequisites

- You need two separate terminal windows.
- You should be in the root directory of the `lanFileSharer/server` project.

## Test Steps

### Step 1: Start the Log Monitor

In the first terminal, start monitoring the `debug.log` file. This file will show us the `sender`'s view of the network in real-time.

```sh
# For Windows (using PowerShell)
Get-Content -Path debug.log -Wait

# For Linux or macOS
tail -f debug.log
```

Keep this terminal open and visible.

### Step 2: Start the Receiver

In the second terminal, start the `receiver` process.

```sh
go run ./cmd/lanfilesharer receive
```

The receiver is now running and broadcasting its presence on the network.

### Step 3: Start the Sender

In a third terminal (or you can reuse the second one after starting the receiver), start the `sender` process.

```sh
go run ./cmd/lanfilesharer send
```

### Step 4: Observe the Discovery

Switch back to your **first terminal** (the log monitor). You should see log messages from the sender appearing every 5 seconds. Initially, it will find 0 services, but after a few seconds, it should discover the receiver.

The log output should look something like this:

```
Discovery Update: Found 1 services.
  - Service: My-PC-Receiver-xxxx, Addr: 192.168.1.10, Port: 8080
```

Wait for this message to appear. This confirms that the sender has successfully discovered the receiver.

### Step 5: Shut Down the Receiver and Measure the Delay

1.  Go to the **second terminal** where the `receiver` is running.
2.  Press `Ctrl+C` to shut down the receiver process.
3.  **Immediately** switch back to the **first terminal** (the log monitor) and start a stopwatch or simply watch the clock.
4.  Continue to observe the log output. The sender will **continue to report "Found 1 services"** for a period of time.
5.  Wait until the log message changes to **`Discovery Update: Found 0 services.`**.
6.  **Record the time** that elapsed between shutting down the receiver and the log message changing.

## Expected Result (The Problem)

You will likely observe a significant delay, potentially **several minutes**, before the sender's log reflects that the receiver has disappeared. This delay is the problem we aim to solve.

## After Implementing a Fix

After a fix (like the active health check) is implemented, you can run this test again. The expected result would be that the sender detects the offline status much faster, ideally within a few seconds.
