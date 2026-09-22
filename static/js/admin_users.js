document.addEventListener('alpine:init', () => {
    Alpine.data('adminUsersManagement', () => ({
        showAddUserForm: false,
        editUserModalOpen: false,
        editUser: {},

        openEditUser(dataset) {
            this.editUser = {
                id: dataset.id,
                first_name: dataset.firstName,
                last_name: dataset.lastName,
                email: dataset.email,
                role: dataset.role,
                oidc_sub: dataset.oidcSub
            };
            this.editUserModalOpen = true;
        }
    }));
});