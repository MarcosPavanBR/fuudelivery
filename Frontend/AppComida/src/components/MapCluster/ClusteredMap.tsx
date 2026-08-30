import React, { useMemo } from 'react';
import { StyleSheet } from 'react-native';
import MapLibreGL from '@maplibre/maplibre-react-native';

interface Delivery {
  id: string;
  latitude: number;
  longitude: number;
  status: 'pending' | 'assigned' | 'in_progress' | 'delivered';
  customerName?: string;
}

interface ClusteredMapProps {
  deliveries: Delivery[];
  userLocation?: {
    latitude: number;
    longitude: number;
  };
  onDeliveryPress?: (delivery: Delivery) => void;
  style?: any;
}

/**
 * Componente de mapa com clusterização de marcadores
 * Otimização de performance para mostrar centenas de entregas sem travar o app
 * 
 * Benefícios:
 * - Renderização nativa via OpenGL (não usa ponte React Native)
 * - Clusterização automática baseada no zoom
 * - Atualizações eficientes do viewport
 * - Suporte a animações nativas
 */
export const ClusteredMap: React.FC<ClusteredMapProps> = ({
  deliveries,
  userLocation,
  onDeliveryPress,
  style,
}) => {
  // Converte deliveries para GeoJSON (formato otimizado para MapLibre)
  const geoJSON = useMemo(() => {
    const features = deliveries.map((delivery) => ({
      type: 'Feature' as const,
      properties: {
        id: delivery.id,
        status: delivery.status,
        customerName: delivery.customerName,
        // Cores diferentes por status
        color: getStatusColor(delivery.status),
      },
      geometry: {
        type: 'Point' as const,
        coordinates: [delivery.longitude, delivery.latitude],
      },
    }));

    return {
      type: 'FeatureCollection' as const,
      features,
    };
  }, [deliveries]);

  // Configuração do cluster
  const clusterLayers = useMemo(
    () => [
      // Camada de clusters (círculos agrupados)
      {
        id: 'clusters',
        type: 'circle' as const,
        source: 'deliveries',
        filter: ['has', 'point_count'] as any,
        paint: {
          'circle-color': [
            'step',
            ['get', 'point_count'],
            '#51bbd6', // 0-100 pontos: azul claro
            100, '#f1f075', // 100-750 pontos: amarelo
            750, '#f28cb1', // 750+ pontos: rosa
          ],
          'circle-radius': [
            'step',
            ['get', 'point_count'],
            20, // 0-100 pontos
            100, 30, // 100-750 pontos
            750, 40, // 750+ pontos
          ],
        },
      },
      // Contador de pontos no cluster
      {
        id: 'cluster-count',
        type: 'symbol' as const,
        source: 'deliveries',
        filter: ['has', 'point_count'] as any,
        layout: {
          'text-field': '{point_count_abbreviated}',
          'text-size': 12,
        },
        paint: {
          'text-color': '#000000',
        },
      },
      // Marcadores individuais (quando zoom está alto)
      {
        id: 'unclustered-points',
        type: 'circle' as const,
        source: 'deliveries',
        filter: ['!', ['has', 'point_count']] as any,
        paint: {
          'circle-color': ['get', 'color'],
          'circle-radius': 8,
          'circle-stroke-width': 2,
          'circle-stroke-color': '#ffffff',
        },
      },
    ],
    []
  );

  const handlePress = (event: any) => {
    if (!onDeliveryPress) return;

    const { features } = event;
    if (features && features.length > 0) {
      const feature = features[0];
      const deliveryId = feature.properties?.id;
      
      if (deliveryId) {
        const delivery = deliveries.find((d) => d.id === deliveryId);
        if (delivery) {
          onDeliveryPress(delivery);
        }
      }
    }
  };

  return (
    <MapLibreGL.MapView
      style={[styles.map, style]}
      logoEnabled={false}
      compassEnabled={true}
      pitchEnabled={false}
      rotateEnabled={false}
    >
      <MapLibreGL.Camera
        zoomLevel={12}
        centerCoordinate={
          userLocation
            ? [userLocation.longitude, userLocation.latitude]
            : [-46.6520646, -23.5648985] // São Paulo default
        }
        animationDuration={1000}
      />

      {/* Localização do usuário */}
      {userLocation && (
        <MapLibreGL.PointAnnotation
          id="user-location"
          coordinate={[userLocation.longitude, userLocation.latitude]}
        >
          <MapLibreGL.Image
            source={require('../assets/user-marker.png')}
            style={{ width: 40, height: 40 }}
          />
        </MapLibreGL.PointAnnotation>
      )}

      {/* Entregas com clusterização */}
      <MapLibreGL.ShapeSource
        id="deliveries"
        shape={geoJSON}
        cluster={true}
        clusterMaxZoom={14}
        clusterRadius={50}
        onPress={handlePress}
      >
        {clusterLayers.map((layer) => (
          <MapLibreGL.CircleLayer key={layer.id} id={layer.id} existing={false} {...layer} />
        ))}
      </MapLibreGL.ShapeSource>
    </MapLibreGL.MapView>
  );
};

// Função auxiliar para cores por status
function getStatusColor(status: Delivery['status']): string {
  switch (status) {
    case 'pending':
      return '#FFA500'; // Laranja
    case 'assigned':
      return '#FFFF00'; // Amarelo
    case 'in_progress':
      return '#0000FF'; // Azul
    case 'delivered':
      return '#00FF00'; // Verde
    default:
      return '#808080'; // Cinza
  }
}

const styles = StyleSheet.create({
  map: {
    flex: 1,
  },
});

export default ClusteredMap;
