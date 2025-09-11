# Caltrans KML Test Data

This directory contains timestamped snapshots of Caltrans KML feeds for testing purposes.

## File Structure

- `*_YYYYMMDD_HHMMSS.kml` - Timestamped snapshots of KML feeds
- `*.kml` - Symlinks pointing to the most recent snapshots (used by tests)

## Usage

### Fetch New Test Data

To capture a new snapshot of the current Caltrans feeds:

```bash
make fetch-test-data
```

This creates timestamped files like:
- `lane_closures_20250911_110046.kml`
- `chp_incidents_20250911_110046.kml`
- `chain_controls_20250911_110046.kml`

### Update Test Data

To update the symlinks to use the latest snapshots:

```bash
make use-latest-test-data
```

This updates the symlinks:
- `lane_closures.kml` → `lane_closures_YYYYMMDD_HHMMSS.kml`
- `chp_incidents.kml` → `chp_incidents_YYYYMMDD_HHMMSS.kml`
- `chain_controls.kml` → `chain_controls_YYYYMMDD_HHMMSS.kml`

### Run Offline Tests

To run tests using local snapshots instead of live feeds:

```bash
make test-caltrans OFFLINE=1
```

## Benefits

1. **Fast Testing**: Offline tests run in ~250ms vs several seconds for live API calls
2. **Consistent Results**: Tests use fixed data sets for reproducible results
3. **Historical Data**: Multiple snapshots allow testing edge cases and variations
4. **No External Dependencies**: Tests don't fail due to network issues or API changes

## Feed Sources

- **Lane Closures**: https://quickmap.dot.ca.gov/data/lcs2way.kml
- **CHP Incidents**: https://quickmap.dot.ca.gov/data/chp-only.kml  
- **Chain Controls**: https://quickmap.dot.ca.gov/data/cc.kml

## Example Usage

```bash
# Capture current feed state during interesting conditions (storms, incidents, etc.)
make fetch-test-data

# Run comprehensive tests with geographic filtering
make test-caltrans OFFLINE=1 FILTER=1 LAT=37.7749 LON=-122.4194 RADIUS=10000

# Test specific feed types
make test-caltrans OFFLINE=1 FEED=lanes
make test-caltrans OFFLINE=1 FEED=chp
```

This approach allows us to build a library of test cases covering various real-world scenarios and edge cases discovered over time.