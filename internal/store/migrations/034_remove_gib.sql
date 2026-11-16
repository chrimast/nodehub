UPDATE host_traffic SET display_unit='GB' WHERE display_unit<>'GB';
UPDATE tenants SET traffic_display_unit='GB' WHERE traffic_display_unit<>'GB';