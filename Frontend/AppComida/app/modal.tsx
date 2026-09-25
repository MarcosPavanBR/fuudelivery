import React, { useState } from "react";
import {
  View,
  StyleSheet,
  Text,
  Image,
  TouchableOpacity,
  TextInput,
  Platform,
} from "react-native";
import { useNavigation, useLocalSearchParams } from "expo-router";
import { useSafeAreaInsets } from "react-native-safe-area-context";

import Colors from "@/constants/Colors";
import Texts from "@/constants/Texts";
import QuantitySelector from "@/components/QuantitySelector";
import AdditionalList from "@/components/AdditionalList";
import helpers from "@/helpers/helpers";
import { useCartApi } from "@/contexts/ApiCartContext";

const ProductPage = () => {
  const params = useLocalSearchParams();
  const navigation = useNavigation();
  const { addCart, editCart } = useCartApi();

  const {
    item,
    title,
    quantityInit = 1,
    selectedsInit = [],
    noteInit = "",
    itemId,
  }: any = params;

  // Item pausado pela loja (esgotado): dá para ver, não para pedir. O
  // servidor também recusa o pedido — isto só evita a surpresa no fim.
  const soldOut = item?.Available === false;

  const [quantity, setQuantity] = useState(quantityInit);
  const [note, setNote] = useState<string>(noteInit);
  const [selectedsAdditional, setSelectedAdditionals] =
    useState<number[]>(selectedsInit);

  const addRemove = (id: number) => {
    // Verifique se o ID já está na lista de selecionados
    if (selectedsAdditional.includes(id)) {
      // Remova-o
      setSelectedAdditionals(selectedsAdditional.filter((e) => e !== id));
    } else {
      // Adicione-o
      setSelectedAdditionals([...selectedsAdditional, id]);
    }
  };

  const calculateFinalPrice = () => {
    // Calcula o preço total dos adicionais selecionados
    const additionalPricesSum = selectedsAdditional.reduce(
      (sum, additionalId) => {
        const additional = item.Additional.find(
          (a: any) => a.ID === additionalId
        );
        return sum + (additional?.Price || 0);
      },
      0
    );

    // Calcula o preço final considerando a quantidade e o preço base do item
    const finalPrice = quantity * (item.Price + additionalPricesSum);

    return finalPrice;
  };

  React.useLayoutEffect(() => {
    navigation.setOptions({
      title: title ?? item.Name,
    });
  }, [navigation]);

  const insets = useSafeAreaInsets();

  return (
    <View style={styles.container}>
      <View>
        {item.Image ? (
          <Image source={{ uri: item.Image }} style={styles.productImage} />
        ) : null}
        <View
          style={{
            ...styles.productInfoContainer,
            marginTop: !item.Image ? 20 : undefined,
          }}
        >
          <Text style={styles.productName}>{item.Name}</Text>
          <Text style={styles.productDescription}>{item.Description}</Text>
          <Text style={styles.productPrice}>
            {helpers.formatCurrency(item.Price)}
          </Text>
        </View>
        {item.Additional && item.Additional.length > 0 ? (
          <AdditionalList
            additionals={item.Additional}
            selected={selectedsAdditional}
            onChange={addRemove}
          />
        ) : null}
        {!soldOut ? (
          <View style={styles.noteContainer}>
            <Text style={styles.noteLabel}>Alguma observação?</Text>
            <TextInput
              value={note}
              onChangeText={setNote}
              placeholder="Ex.: sem cebola, ponto da carne..."
              placeholderTextColor={Colors.light.secondaryText}
              maxLength={140}
              multiline
              style={styles.noteInput}
            />
            <Text style={styles.noteCounter}>{note.length}/140</Text>
          </View>
        ) : null}
      </View>
      <View style={{ ...styles.mainContainer, paddingBottom: insets.bottom }}>
        <QuantitySelector
          quantity={quantity}
          onIncrement={() => setQuantity(quantity + 1)}
          onDecrement={() =>
            setQuantity((quantity: number) =>
              quantity !== 1 ? quantity - 1 : 1
            )
          }
        />
        <TouchableOpacity
          style={{ ...styles.btns, opacity: soldOut ? 0.5 : 1 }}
          disabled={soldOut}
          onPress={() => {
            const trimmed = note.trim();
            const entry = {
              item,
              additionals: selectedsAdditional,
              quantity,
              ...(trimmed ? { note: trimmed } : {}),
            };
            !itemId ? addCart(entry) : editCart({ ...entry, id: itemId });
            navigation.goBack();
          }}
        >
          <Text style={{ fontWeight: "500", color: Colors.light.white }}>
            {soldOut ? "Esgotado" : !itemId ? Texts.add : Texts.alter}
          </Text>
          <Text style={{ fontWeight: "500", color: Colors.light.white }}>
            {helpers.formatCurrency(calculateFinalPrice())}
          </Text>
        </TouchableOpacity>
      </View>
    </View>
  );
};

const styles = StyleSheet.create({
  container: {
    flex: 1,
    flexDirection: "column",
    justifyContent: "space-between",
    backgroundColor: Colors.light.white,
    paddingBottom: Platform.OS === "android" ? 10 : null,
  },
  mainContainer: {
    flexDirection: "row",
    justifyContent: "space-between",
    padding: 10,

    backgroundColor: Colors.light.background,
  },
  btns: {
    width: "65%",
    backgroundColor: Colors.light.tint,
    paddingLeft: 10,
    paddingRight: 10,
    borderRadius: 3,
    flexDirection: "row",
    justifyContent: "space-between",
    alignContent: "center",
    alignItems: "center",
  },
  productImage: {
    width: "100%",
    height: 200,
    resizeMode: "cover",
  },
  productInfoContainer: {
    padding: 16,
  },
  productName: {
    fontSize: 24,
    fontWeight: "400",
    marginBottom: 8,
  },
  productDescription: {
    fontSize: 16,
    marginBottom: 16,
    color: Colors.light.secondaryText,
  },
  productPrice: {
    fontSize: 18,
    fontWeight: "400",
    color: "green",
  },
  noteContainer: {
    paddingHorizontal: 16,
    paddingTop: 12,
  },
  noteLabel: {
    fontSize: 14,
    fontWeight: "500",
    marginBottom: 6,
    color: Colors.light.text,
  },
  noteInput: {
    minHeight: 60,
    borderWidth: 1,
    borderColor: Colors.light.tabIconDefault,
    borderRadius: 6,
    padding: 10,
    fontSize: 14,
    textAlignVertical: "top",
    color: Colors.light.text,
  },
  noteCounter: {
    alignSelf: "flex-end",
    fontSize: 11,
    marginTop: 4,
    color: Colors.light.secondaryText,
  },
});

export default ProductPage;
