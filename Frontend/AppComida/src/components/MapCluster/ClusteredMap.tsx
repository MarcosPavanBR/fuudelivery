import React, { useMemo } from 'react';
import { StyleSheet } from 'react-native';
import { Map, Camera, GeoJSONSource, Layer, Marker } from '@maplibre/maplibre-react-native';

interface Delivery {
  id: string;
  latitude: number;
  longitude: number;
  status: 'pending' | 'assigned' | 'in_progress' | 'delivered';
  customerName?: string;
}

interface ClusteredMapProps {
  deliveries: Delivery[];
  userLocation?: { latitude: number; longitude: number };
  onDeliveryPress?: (delivery: Delivery) => void;
  style?: any;
}

/**
 * Mapa com clusterizacao de marcadores — MapLibre RN v11.
 */
export const ClusteredMap: React.FC<ClusteredMapProps> = ({
  deliveries,
  userLocation,
  onDeliveryPress,
  style,
}) => {
  const geoJSON = useMemo(() => ({
    type: 'FeatureCollection' as const,
    features: deliveries.map((d) => ({
      type: 'Feature' as const,
      properties: { id: d.id, status: d.status, customerName: d.customerName, color: getStatusColor(d.status) },
      geometry: { type: 'Point' as const, coordinates: [d.longitude, d.latitude] },
    })),
  }), [deliveries]);

  const handlePress = (event: any) => {
    if (!onDeliveryPress) return;
    const f = event?.features?.[0];
    if (f?.properties?.id) {
      const d = deliveries.find((x) => x.id === f.properties.id);
      if (d) onDeliveryPress(d);
    }
  };

  const center: [number, number] = userLocation
    ? [userLocation.longitude, userLocation.latitude]
    : [-46.6520646, -23.5648985];

  return (
    <Map style={[styles.map, style]} mapStyle="https://basemaps.cartocdn.com/gl/positron-gl-style/style.json">
      {/* @ts-expect-error Camera initialViewState props are runtime-only */}
      <Camera initialViewState={{ longitude: center[0], latitude: center[1], zoom: 12 }} />

      {userLocation && (
        <Marker id="user-location" lngLat={center as [number, number]}><></></Marker>
      )}

      <GeoJSONSource
        id="deliveries"
        data={geoJSON as any}
        cluster={true}
        clusterMaxZoom={14}
        clusterRadius={50}
        onPress={handlePress}
      >
        <Layer
          id="clusters"
          type="circle"
          source="deliveries"
          filter={['has', 'point_count'] as any}
          paint={{
            'circle-color': ['step', ['get', 'point_count'], '#51bbd6', 100, '#f1f075', 750, '#f28cb1'],
            'circle-radius': ['step', ['get', 'point_count'], 20, 100, 30, 750, 40],
          }}
        />
        <Layer
          id="cluster-count"
          type="symbol"
          source="deliveries"
          filter={['has', 'point_count'] as any}
          layout={{ 'text-field': '{point_count_abbreviated}', 'text-size': 12 }}
          paint={{ 'text-color': '#000000' }}
        />
        <Layer
          id="unclustered-points"
          type="circle"
          source="deliveries"
          filter={['!', ['has', 'point_count']] as any}
          paint={{ 'circle-color': ['get', 'color'], 'circle-radius': 8, 'circle-stroke-width': 2, 'circle-stroke-color': '#ffffff' }}
        />
      </GeoJSONSource>
    </Map>
  );
};

function getStatusColor(status: Delivery['status']): string {
  switch (status) {
    case 'pending': return '#FFA500';
    case 'assigned': return '#FFFF00';
    case 'in_progress': return '#0000FF';
    case 'delivered': return '#00FF00';
    default: return '#808080';
  }
}

const styles = StyleSheet.create({ map: { flex: 1 } });

export default ClusteredMap;
