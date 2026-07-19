import React, { useEffect, useState } from "react";
import Modal from "react-modal";
import { useAuth } from "../context/AuthContext";
import { FiEdit, FiSave, FiX } from "react-icons/fi";
import { MdDeleteOutline } from "react-icons/md";
import { toast } from "react-toastify";
import Texts from "../constants/Texts";
import additionalsModel from "../services/additionals.model";
import Strings from "../constants/Strings";
import categoryModel from "../services/category.model";
import helper from "../helpers/helper";

Modal.setAppElement("#root");

const ModalAddItens = ({
  isOpen,
  onClose,
  item,
  isCategory,
  onRefreshItens,
}) => {
  const [items, setItems] = useState([]);
  const { getUser } = useAuth();
  const [selectedItems, setSelectedItems] = useState([]);
  const [editItem, setEditItem] = useState(null);

  const init = async () => {
    const myid = getUser().id;
    setItems(
      isCategory
        ? await categoryModel.getCategories(myid)
        : await additionalsModel.getAdditionals(myid)
    );
  };

  async function saveItem() {
    const isCreate = !editItem.ID || editItem.ID === Strings.id_default;
    let finalItem = null;
    if (!isCategory) {
      finalItem = isCreate
        ? await additionalsModel.createAdditional(items, editItem, getUser().id)
        : await additionalsModel.updateAdditional(items, editItem);
      if (!finalItem) { toast.error(Texts.erro_cardapio); return; }
    } else {
      finalItem = isCreate
        ? await categoryModel.createCategory(items, editItem, getUser().id)
        : await categoryModel.updateCategory(items, editItem, getUser().id);
    }
    const tag = isCategory ? "Categories" : "Additional";
    if (item[tag].find((e) => e.ID === editItem.ID))
      await onRefreshItens({
        ...item,
        [tag]: selectedItems.map((e) => (e.ID === editItem.ID ? editItem : e)),
      });
    setItems(finalItem);
    toast.success(Texts.alteracao_aplicada);
  }

  function editId(id) {
    if (!id) { setEditItem(null); }
    else {
      const myItem = items.find((e) => e.ID === id);
      if (myItem) setEditItem(myItem);
    }
    setItems(items.map((e) => ({ ...e, edit: e.ID == id })));
  }

  const handleRemoveItem = (itemToRemove) => {
    const updatedItems = selectedItems.filter((it) => it.ID !== itemToRemove.ID);
    setSelectedItems(updatedItems);
    return updatedItems;
  };

  const handlerItem = async (it) => {
    let finalItems = selectedItems;
    const resp = isCategory
      ? await categoryModel.handlerVinculoProdutoCategoria(item.ID, it.ID)
      : await additionalsModel.handlerVinculoProdutoAdicional(item.ID, it.ID);
    if (!selectedItems.find((a) => a.ID === it.ID)) {
      finalItems = [...selectedItems, it];
      setSelectedItems(finalItems);
    } else {
      finalItems = handleRemoveItem(it);
    }
    if (resp) {
      const tag = isCategory ? "Categories" : "Additional";
      await onRefreshItens({ ...item, [tag]: finalItems });
      toast.success(Texts.adicionado_no_produto);
    }
  };

  const handlerEditItem = (target) => {
    setEditItem({ ...editItem, [target.name]: target.value });
  };

  const newItem = () => {
    const myNew = Strings.initial_order({ ID: Strings.id_default, Name: Texts.novo_produto });
    setItems([{ ...myNew, edit: true }, ...items.map((e) => ({ ...e, edit: false }))]);
    setEditItem({ ...myNew });
  };

  const removeNewItem = () => {
    setItems(items.filter((e) => e.ID !== Strings.id_default));
  };

  const deleteItem = async (id) => {
    const newItem = isCategory
      ? await categoryModel.deleteCategory(items, id)
      : await additionalsModel.deleteAdditional(items, id);
    if (newItem) {
      toast.success(Texts.alteracao_aplicada);
      setItems(newItem);
      const tag = isCategory ? "Categories" : "Additional";
      await onRefreshItens({ ...item, [tag]: item[tag].filter((e) => e.ID !== id) });
    } else {
      toast.error(Texts.erro_cardapio);
    }
  };

  useEffect(() => {
    init();
    const final = isCategory ? item.Categories : item.Additional;
    setSelectedItems(final ?? []);
  }, [isCategory, item]);

  return (
    <Modal
      isOpen={isOpen}
      onRequestClose={() => { onClose(); editId(null); removeNewItem(); }}
      overlayClassName="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4"
      style={{ content: { outline: "none" } }}
    >
      <div className="bg-white rounded-2xl w-full max-w-3xl max-h-[85vh] overflow-hidden shadow-modal animate-slide-up">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-100">
          <h4 className="text-lg font-bold text-gray-900">
            {isCategory ? "Categoria" : "Adicionais"}
          </h4>
          <button onClick={onClose} className="p-2 rounded-xl hover:bg-gray-100 transition-colors">
            <FiX className="h-5 w-5 text-gray-500" />
          </button>
        </div>

        {/* Search + Add */}
        <div className="px-6 py-4 flex gap-3">
          <input
            type="text"
            placeholder={Texts.search_placeholer}
            className="flex-1 px-4 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white"
          />
          <button
            type="button"
            onClick={() => newItem()}
            disabled={items.find((e) => e.ID === Strings.id_default)}
            className="flex items-center justify-center px-4 py-2.5 rounded-xl text-white text-sm font-semibold disabled:opacity-50"
            style={{ background: "linear-gradient(135deg, #EA1D2C, #C41420)" }}
          >
            <FiX className="h-5 w-5 rotate-45" />
          </button>
        </div>

        {/* Table */}
        <div className="px-6 pb-6 overflow-y-auto max-h-[60vh]">
          <table className="w-full">
            <thead>
              <tr className="border-b border-gray-100">
                <th className="text-left py-3 px-3 text-xs font-semibold text-gray-500 uppercase">{Texts.id}</th>
                <th className="text-left py-3 px-3 text-xs font-semibold text-gray-500 uppercase">{Texts.acoes}</th>
                <th className="text-left py-3 px-3 text-xs font-semibold text-gray-500 uppercase">{Texts.nome}</th>
                {!isCategory && <th className="text-left py-3 px-3 text-xs font-semibold text-gray-500 uppercase">{Texts.preco}</th>}
                <th className="text-left py-3 px-3 text-xs font-semibold text-gray-500 uppercase">Vincular?</th>
              </tr>
            </thead>
            <tbody>
              {items.map((myItem) => (
                <tr key={myItem.ID} className="border-b border-gray-50 hover:bg-gray-50 transition-colors">
                  <td className="py-3 px-3 text-sm font-medium" style={{ color: "#EA1D2C" }}>
                    {myItem.ID !== Strings.id_default ? myItem.ID : "-"}
                  </td>
                  <td className="py-3 px-3">
                    {!myItem.edit ? (
                      <div className="flex gap-2">
                        <button onClick={() => editId(myItem.ID)} className="p-1.5 rounded-lg hover:bg-gray-100 text-gray-500">
                          <FiEdit size={16} />
                        </button>
                        <button onClick={() => deleteItem(myItem.ID)} className="p-1.5 rounded-lg hover:bg-red-50 text-red-500">
                          <MdDeleteOutline size={18} />
                        </button>
                      </div>
                    ) : (
                      <div className="flex gap-2">
                        <button onClick={() => { editId(null); if (myItem.ID == Strings.id_default) removeNewItem(); }} className="p-1.5 rounded-lg hover:bg-gray-100 text-gray-500">
                          <FiX size={16} />
                        </button>
                        <button onClick={() => saveItem()} className="p-1.5 rounded-lg hover:bg-green-50 text-green-600">
                          <FiSave size={16} />
                        </button>
                      </div>
                    )}
                  </td>
                  <td className="py-3 px-3">
                    {!myItem.edit ? (
                      <span className="text-sm font-medium text-gray-900">{myItem.Name}</span>
                    ) : (
                      <input type="text" value={editItem?.Name} name="Name" onChange={({ target }) => handlerEditItem(target)} className="w-full px-3 py-1.5 bg-gray-50 border border-gray-200 rounded-lg text-sm" />
                    )}
                  </td>
                  {!isCategory && (
                    <td className="py-3 px-3">
                      {!myItem.edit ? (
                        <span className="text-sm font-medium text-gray-900">{helper.formatCurrency(myItem.Price)}</span>
                      ) : (
                        <input type="number" min={0} value={editItem?.Price ?? 0} name="Price" onChange={({ target }) => handlerEditItem(target)} className="w-full px-3 py-1.5 bg-gray-50 border border-gray-200 rounded-lg text-sm" />
                      )}
                    </td>
                  )}
                  <td className="py-3 px-3">
                    <input
                      type="radio"
                      disabled={myItem.ID === Strings.id_default}
                      className="w-5 h-5 accent-green-600"
                      checked={selectedItems.some((sel) => sel.ID === myItem.ID)}
                      onClick={() => handlerItem(myItem)}
                    />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </Modal>
  );
};

export default ModalAddItens;
