# BuildSim Error Handling Issues

Reference list of identified error handling gaps in the BuildSim server.

---

## Equipment

### 1. No validation on required fields at create
**File:** `pkg/server/handlers/equipment.go:23`  
**Issue:** Only `id` is checked. `name`, `type`, `category`, `level`, `room` are never validated — garbage data gets stored.  
**Test:**
```powershell
$body = '{"id":"eq-bad"}'; Invoke-RestMethod -Uri http://localhost:9090/api/equipment -Method POST -ContentType "application/json" -Body $body
```
**Expected:** `400 Bad Request`  
**Actual:** `201 Created`

---

### 2. Bulk create silently skips failed items
**File:** `pkg/server/handlers/equipment.go:88-95`  
**Issue:** Items with missing ID or duplicate ID are skipped with `continue`. Response only says `created: 1, total: 3` — no detail on what failed or why.  
**Test:**
```powershell
$body = '[{"id":"eq-bulk-1","name":"Valid","type":"sensor","category":"hvac","level":"level0","room":"R001"},{"id":"","name":"No ID","type":"sensor","category":"hvac","level":"level0","room":"R001"},{"id":"eq-1","name":"Dup","type":"sensor","category":"hvac","level":"level0","room":"R001"}]'; Invoke-RestMethod -Uri http://localhost:9090/api/equipment/bulk -Method POST -ContentType "application/json" -Body $body
```
**Expected:** response includes `errors` array listing rejected items and reasons  
**Actual:** `{"created":1,"total":3,"version":...}` — no error detail

---

### 3. Notify bumps version unconditionally
**File:** `pkg/server/handlers/equipment.go:104`  
**Issue:** `/api/equipment/notify` bumps the version and broadcasts to all clients even when nothing changed. Every connected browser re-fetches unnecessarily.  
**Test:**
```powershell
Invoke-RestMethod -Uri http://localhost:9090/api/equipment/notify -Method POST
Invoke-RestMethod -Uri http://localhost:9090/api/equipment/notify -Method POST
```
**Expected:** no-op or validation that equipment exists  
**Actual:** version increments on every call

---

## Sensors

### 4. String-based error classification (brittle)
**File:** `pkg/server/handlers/sensor.go:28`  
**Issue:** HTTP status code is chosen by comparing the store's error string: `if errMsg == "sensor already exists"`. If the store ever changes its wording, the status code silently breaks.  
**Test:**
```powershell
$body = '{"id":"sen-1","name":"Temp","dataType":"text"}'; Invoke-RestMethod -Uri http://localhost:9090/api/equipment/eq-1/sensors -Method POST -ContentType "application/json" -Body $body
```
```powershell
try { Invoke-RestMethod -Uri http://localhost:9090/api/equipment/eq-1/sensors -Method POST -ContentType "application/json" -Body $body } catch { $_.ErrorDetails.Message }
```
**Expected:** `409 Conflict` backed by a sentinel error type, not a string match  
**Actual:** works today but breaks silently if store error message changes

---

### 5. Dangling pointer after slice reallocation
**File:** `pkg/store/memory.go:209`  
**Issue:** `s.sensors[sen.ID] = &e.Sensors[len(e.Sensors)-1]` stores a pointer into a slice. When the slice grows and reallocates, this pointer points to the old backing array. `SetSensorValue` writes to the stale pointer — the equipment's sensor list is not updated.  
**Test:**
```powershell
$body = '{"id":"eq-ptr","name":"Ptr","type":"sensor","category":"hvac","level":"level0","room":"R001"}'; Invoke-RestMethod -Uri http://localhost:9090/api/equipment -Method POST -ContentType "application/json" -Body $body
$body = '{"id":"sen-0","name":"Sensor 0","dataType":"text"}'; Invoke-RestMethod -Uri http://localhost:9090/api/equipment/eq-ptr/sensors -Method POST -ContentType "application/json" -Body $body
$body = '{"id":"sen-1","name":"Sensor 1","dataType":"text"}'; Invoke-RestMethod -Uri http://localhost:9090/api/equipment/eq-ptr/sensors -Method POST -ContentType "application/json" -Body $body
$body = '{"id":"sen-2","name":"Sensor 2","dataType":"text"}'; Invoke-RestMethod -Uri http://localhost:9090/api/equipment/eq-ptr/sensors -Method POST -ContentType "application/json" -Body $body
$body = '{"id":"sen-3","name":"Sensor 3","dataType":"text"}'; Invoke-RestMethod -Uri http://localhost:9090/api/equipment/eq-ptr/sensors -Method POST -ContentType "application/json" -Body $body
$body = '{"id":"sen-4","name":"Sensor 4","dataType":"text"}'; Invoke-RestMethod -Uri http://localhost:9090/api/equipment/eq-ptr/sensors -Method POST -ContentType "application/json" -Body $body
$body = '{"id":"sen-5","name":"Sensor 5","dataType":"text"}'; Invoke-RestMethod -Uri http://localhost:9090/api/equipment/eq-ptr/sensors -Method POST -ContentType "application/json" -Body $body
$body = '{"dataType":"text","value":"UPDATED"}'; Invoke-RestMethod -Uri http://localhost:9090/api/sensors/sen-0/value -Method PUT -ContentType "application/json" -Body $body
Invoke-RestMethod -Uri http://localhost:9090/api/equipment/eq-ptr/sensors
```
**Expected:** `sen-0` shows `"value":"UPDATED"`  
**Actual:** `sen-0` may still show `"value":""` — stale pointer

---

### 6. Identical 404 for two different failures in SetSensorValue
**File:** `pkg/store/memory.go:243-263`  
**Issue:** Both "sensor never existed" and "sensor in index but missing from equipment slice" return `{"error":"sensor not found"}`. Impossible to distinguish the two cases.  
**Test:**
```powershell
try { Invoke-RestMethod -Uri http://localhost:9090/api/sensors/ghost-sensor/value -Method PUT -ContentType "application/json" -Body '{"dataType":"text","value":"42"}' } catch { $_.ErrorDetails.Message }
```
**Expected:** distinct error messages for each failure mode  
**Actual:** generic `{"error":"sensor not found"}` in both cases

---

## Actuators

### 7. String-based error classification (brittle) — same as sensors
**File:** `pkg/server/handlers/actuator.go:28`  
**Issue:** Same pattern as sensor handler — `if errMsg == "actuator already exists"` to decide status code.  
**Test:**
```powershell
$body = '{"id":"eq-act","name":"Actuator Eq","type":"actuator","category":"hvac","level":"level0","room":"R001"}'; Invoke-RestMethod -Uri http://localhost:9090/api/equipment -Method POST -ContentType "application/json" -Body $body
$body = '{"id":"act-1","name":"Valve"}'; Invoke-RestMethod -Uri http://localhost:9090/api/equipment/eq-act/actuators -Method POST -ContentType "application/json" -Body $body
try { Invoke-RestMethod -Uri http://localhost:9090/api/equipment/eq-act/actuators -Method POST -ContentType "application/json" -Body $body } catch { $_.ErrorDetails.Message }
```
**Expected:** `409 Conflict` backed by sentinel error  
**Actual:** works today, breaks silently if store wording changes

---

## Graph / Routing

### 8. Missing route parameters give cryptic internal error
**File:** `pkg/server/handlers/graph.go:76-87`  
**Issue:** Empty `from`/`to` params cause `strconv.Atoi("")` to fail internally. The error message leaks the implementation detail instead of saying the parameter is missing.  
**Test:**
```powershell
Invoke-RestMethod -Uri "http://localhost:9090/api/graph/route?level=level0&from=&to="
```
**Expected:** `400 Bad Request` — `"'from' parameter is required"`  
**Actual:** `400 Bad Request` — `"invalid 'from' room id"` (cryptic, leaks internals)

---

## Summary Table

| # | Area | File | Severity |
|---|------|------|----------|
| 1 | Equipment create — no field validation | `handlers/equipment.go:23` | High |
| 2 | Bulk create — silent skip, no error detail | `handlers/equipment.go:88` | High |
| 3 | Notify — unconditional version bump | `handlers/equipment.go:104` | Medium |
| 4 | Sensor add — string-based status mapping | `handlers/sensor.go:28` | Medium |
| 5 | Sensor add — dangling pointer after realloc | `store/memory.go:209` | Critical |
| 6 | SetSensorValue — ambiguous 404 | `store/memory.go:243` | Medium |
| 7 | Actuator add — string-based status mapping | `handlers/actuator.go:28` | Medium |
| 8 | Graph route — cryptic missing param error | `handlers/graph.go:76` | Low |
