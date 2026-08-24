import React, { useEffect, useState } from "react";
import MenuLayout from "../../../components/Menu";
import CardapioList from "../../../components/CardapioList";
import SearchInput from "../../../components/SearchInput";
import AddButton from "../../../components/AddButton";
import Strings from "../../../constants/Strings";
import Texts from "../../../constants/Texts";
import productsModel from "../../../services/products.model";
import { useAuth } from "../../../context/AuthContext";
import { FiLoader, FiPlus } from "react-icons/fi";

const Cardapio = () => {
  const { getUser } = useAuth();
  const [items, setItems] = useState([]);
  const [searchTerm, setSearchTerm] = useState("");
  const [editModalOpen, setEditModalOpen] = useState(false);
  const [load, setLoad] = useState(false);
  const [loadError, setLoadError] = useState(false);
  const [selectedItem, setSelectedItem] = useState(null);

  async function start() {
    setLoad(true);
    try {
      const establishmentId = getUser()?.establishment?.id || getUser().id;
      const products = await productsModel.getProducts(establishmentId);
      setItems(products);
      setLoadError(false);
    } catch (e) {
      setLoadError(true);
    } finally {
      setLoad(false);
    }
  }

  async function onRefreshItens(item) {
    if (item && selectedItem) setSelectedItem(item);
    await start();
  }

  async function save(item) {
    const value = items.map((e) => {
      if (e.ID === item.id) {
        return { ...e, Name: item.name, Description: item.description, Price: item.price, Image: item.image };
      }
      return e;
    });
    setItems(value);
  }

  useEffect(() => {
    start();
  }, []);

  const handleSearch = (term) => setSearchTerm(term);

  return (
    <MenuLayout>
      <div className="mb-6">
        <h3 className="text-lg font-bold text-gray-900">{Texts.gestor_cardapio}</h3>
        <p className="text-sm text-gray-500 mt-1">{Texts.cardapio_desc}</p>
      </div>

      <div className="flex items-center gap-3 mb-6">
        <SearchInput onSearch={handleSearch} />
        <AddButton
          onClick={() => {
            setEditModalOpen(true);
            setSelectedItem(Strings.initial_order());
          }}
        />
      </div>

      {load ? (
        <div className="flex items-center justify-center h-32">
          <FiLoader className="animate-spin h-6 w-6" style={{ color: "#DC2626" }} />
        </div>
      ) : loadError ? (
        <div className="flex flex-col items-center justify-center h-32 gap-3">
          <p className="text-sm text-gray-500">
            Não foi possível carregar o cardápio. Verifique sua conexão.
          </p>
          <button
            onClick={start}
            className="rounded-lg bg-[#DC2626] px-4 py-2 text-sm font-semibold text-white hover:bg-[#B91C1C]"
          >
            Tentar novamente
          </button>
        </div>
      ) : (
        <CardapioList
          items={items.filter((item) =>
            item?.Name?.toLowerCase().includes(searchTerm.toLowerCase())
          )}
          editModalOpen={editModalOpen}
          selectedItem={selectedItem}
          setEditModalOpen={setEditModalOpen}
          setSelectedItem={setSelectedItem}
          onSave={save}
          onRefreshItens={onRefreshItens}
        />
      )}
    </MenuLayout>
  );
};

export default Cardapio;
